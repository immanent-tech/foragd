/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

/*
* Largely based off the Elasticsearch bulk indexer helper, adapted for Opensearch.
*
* https://github.com/elastic/go-elasticsearch/blob/main/esutil/bulk_indexer.go
 */

package bulk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch"
)

// ErrIndexerClosed is returned when Flush is called on a closed indexer.
var ErrIndexerClosed = errors.New("bulk indexer is closed")

type Config struct {
	NumWorkers          int           // The number of workers. Defaults to runtime.NumCPU().
	FlushBytes          int           // The flush threshold in bytes. Defaults to 5MB.
	FlushInterval       time.Duration // The flush threshold as duration. Defaults to 30sec.
	FlushJitter         time.Duration // Max random jitter added to FlushInterval per worker, to avoid lockstep flushes. Defaults to 0 (disabled).
	QueueSizeMultiplier int           // The capacity of each worker's item queue. Total capacity is NumWorkers * QueueSizeMultiplier. Defaults to 1.

	Index  opensearch.Index
	Params *opensearchapi.BulkParams

	Client *opensearch.Client
	Logger *slog.Logger

	OnError      func(context.Context, error)          // Called for indexer errors.
	OnFlushStart func(context.Context) context.Context // Called when the flush starts.
	OnFlushEnd   func(context.Context)                 // Called when the flush ends.
}

type BulkResponse struct {
	*opensearchapi.BulkResp
}

type Indexer struct {
	wg         sync.WaitGroup
	config     Config
	workers    []*worker
	flushMu    sync.Mutex    // serialises Flush() and Close()
	addCounter atomic.Uint64 // round-robin dispatch counter for Add()
	closed     atomic.Bool   // set by Close()
}

func New(cfg Config) (*Indexer, error) {
	// Set defaults if not set.
	if cfg.Client == nil {
		client, err := opensearch.GetClient()
		if err != nil {
			return nil, fmt.Errorf("get client: %w", err)
		}
		cfg.Client = client
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = runtime.NumCPU()
	}

	if cfg.FlushBytes <= 0 {
		cfg.FlushBytes = 5e+6
	}

	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 30 * time.Second
	}

	if cfg.QueueSizeMultiplier <= 0 {
		cfg.QueueSizeMultiplier = 1
	}

	indexer := Indexer{
		config: cfg,
	}

	indexer.init()

	return &indexer, nil
}

func (bi Indexer) GetClient() *opensearch.Client {
	return bi.config.Client
}

// Add adds an item to the indexer.
//
// Adding an item after a call to Close() will panic.
func (bi *Indexer) Add(ctx context.Context, action Action) error {
	// atomic.AddUint64(&bi.stats.numAdded, 1)

	if err := ctx.Err(); err != nil {
		if bi.config.OnError != nil {
			bi.config.OnError(ctx, err)
		}
		return err
	}

	// if item.ctx == nil {
	// 	item.ctx = ctx
	// }

	// Serialize metadata to JSON
	action.encode()
	// Compute length for body & metadata
	if err := action.computeLength(); err != nil {
		return err
	}

	n := bi.addCounter.Add(1) - 1
	numWorkers := uint64(len(bi.workers))
	primary := bi.workers[n%numWorkers]

	// Avoid the O(N) scan when the hashed worker can take the item immediately.
	select {
	case primary.ch <- action:
		return nil
	default:
	}

	// Spread load: prefer making progress on any idle worker over blocking
	// on one busy worker. Start after primary to avoid retrying it and to
	// distribute scan pressure evenly under sustained back-pressure.
	for i := uint64(1); i < numWorkers; i++ {
		w := bi.workers[(n+i)%numWorkers]
		select {
		case w.ch <- action:
			return nil
		default:
		}
	}

	// Back-pressure: all queues full; block until primary drains or the
	// context is cancelled.
	select {
	case <-ctx.Done():
		if bi.config.OnError != nil {
			bi.config.OnError(ctx, ctx.Err())
		}
		return ctx.Err()
	case primary.ch <- action:
	}

	return nil
}

// Close stops the periodic flush, closes the indexer queue channel,
// which triggers the workers to flush and stop.
//
// If the indexer auto-created its client (because BulkIndexerConfig.Client was
// nil), Close also closes that client. It remains the caller's responsibility
// to close any client they passed in via BulkIndexerConfig.Client.
func (bi *Indexer) Close(ctx context.Context) error {
	bi.flushMu.Lock()
	bi.closed.Store(true)
	for _, w := range bi.workers {
		close(w.ch)
	}
	bi.flushMu.Unlock()

	var firstErr error

	if err := ctx.Err(); err != nil {
		if bi.config.OnError != nil {
			bi.config.OnError(ctx, err)
		}
		firstErr = err
	} else {
		done := make(chan struct{})
		go func() {
			bi.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			if bi.config.OnError != nil {
				bi.config.OnError(ctx, ctx.Err())
			}
			firstErr = ctx.Err()
		}
	}

	// if bi.ownedClient != nil {
	// 	if err := bi.ownedClient.Close(ctx); err != nil && firstErr == nil {
	// 		firstErr = err
	// 	}
	// }

	return firstErr
}

// Flush drains all currently queued items, flushes all worker buffers to
// Elasticsearch, and waits for the flushes to complete. The indexer remains
// usable after Flush returns.
func (bi *Indexer) Flush(ctx context.Context) error {
	if bi.closed.Load() {
		return ErrIndexerClosed
	}

	bi.flushMu.Lock()
	defer bi.flushMu.Unlock()

	if bi.closed.Load() {
		return ErrIndexerClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	numWorkers := len(bi.workers)
	dones := make([]chan struct{}, numWorkers)
	for i := range dones {
		dones[i] = make(chan struct{})
	}

	for i, w := range bi.workers {
		sentinel := Action{flushDone: dones[i]}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case w.ch <- sentinel:
		}
	}

	for i := range numWorkers {
		select {
		case <-dones[i]:
		case <-ctx.Done():
			if bi.config.OnError != nil {
				bi.config.OnError(ctx, ctx.Err())
			}
			return ctx.Err()
		}
	}

	return nil
}

// init initializes the bulk indexer.
func (bi *Indexer) init() {
	for i := 1; i <= bi.config.NumWorkers; i++ {
		bi.wg.Add(1)
		w := worker{
			id:     i,
			ch:     make(chan Action, bi.config.QueueSizeMultiplier),
			bi:     bi,
			buf:    bytes.NewBuffer(make([]byte, 0, bi.config.FlushBytes)),
			ticker: time.NewTicker(bi.nextFlushInterval()),
			logger: bi.config.Logger.With(slog.Int("worker_id", i)),
		}
		w.run()
		bi.workers = append(bi.workers, &w)
	}
}

// nextFlushInterval returns FlushInterval plus a random jitter in [0, FlushJitter) when FlushJitter > 0. Callers are
// independent worker goroutines; math/rand's top-level functions are goroutine-safe.
func (bi *Indexer) nextFlushInterval() time.Duration {
	if bi.config.FlushJitter <= 0 {
		return bi.config.FlushInterval
	}
	return bi.config.FlushInterval + time.Duration(rand.Int64N((int64(bi.config.FlushJitter))))
}

type worker struct {
	id     int
	ch     chan Action
	bi     *Indexer
	buf    *bytes.Buffer
	items  []Action
	ticker *time.Ticker
	logger *slog.Logger
}

// run launches the worker in a goroutine.
func (w *worker) run() {
	go func() {
		ctx := context.Background()

		w.logger.Debug("Bulk indexer worker started")

		defer func() {
			w.flush(ctx)
			w.ticker.Stop()
			w.bi.wg.Done()
		}()

		for {
			select {
			case <-w.ticker.C:
				w.logger.Debug(
					"Auto-flushing worker",
					slog.Int("id", w.id),
					slog.Duration("interval", w.bi.config.FlushInterval),
				)
				w.flush(ctx)
			case item, ok := <-w.ch:
				if !ok {
					return
				}

				if item.flushDone != nil {
					w.logger.Debug("Flush sentinel received.")
					w.flush(ctx)
					close(item.flushDone)
					continue
				}

				// if w.bi.config.DebugLogger != nil {
				// 	w.bi.config.DebugLogger.Printf(
				// 		"[worker-%03d] Received item [%s:%s]\n",
				// 		w.id,
				// 		item.Action,
				// 		item.DocumentID,
				// 	)
				// }

				oversizePayload := w.bi.config.FlushBytes <= item.payloadLength
				if !oversizePayload && w.buf.Len() > 0 && w.buf.Len()+item.payloadLength >= w.bi.config.FlushBytes {
					w.flush(ctx)
				}

				_, err := item.WriteTo(w.buf)
				if err != nil {
					w.logger.Warn("Failed to index item", slog.Any("error", err))
					continue
					// if item.OnFailure != nil {
					// 	item.OnFailure(item.ctx, item, BulkIndexerResponseItem{}, err)
					// }
				}

				w.items = append(w.items, item)
				// Should the item payload exceed the configured FlushBytes flush happens instantly.
				if oversizePayload {
					w.logger.Warn("Oversive payload in item. Calling flush.")
					w.flush(ctx)
				}
			}
		}
	}()
}

// flush writes out the worker buffer and handles errors. It also restarts the ticker. Returns true to indicate success.
func (w *worker) flush(ctx context.Context) bool {
	ok := true
	if err := w.flushBuffer(ctx); err != nil {
		ok = false
	}
	w.ticker.Reset(w.bi.nextFlushInterval())
	return ok
}

// flushBuffer writes out the worker buffer.
func (w *worker) flushBuffer(ctx context.Context) error {
	if w.bi.config.OnFlushStart != nil {
		ctx = w.bi.config.OnFlushStart(ctx)
	}

	if w.bi.config.OnFlushEnd != nil {
		defer func() { w.bi.config.OnFlushEnd(ctx) }()
	}

	bufLen := w.buf.Len()

	if bufLen < 1 {
		w.logger.Debug("Flush buffer empty")
		return nil
	}

	defer func() {
		w.items = nil
		if w.buf.Cap() > w.bi.config.FlushBytes {
			w.buf = bytes.NewBuffer(make([]byte, 0, w.bi.config.FlushBytes))
		} else {
			w.buf.Reset()
		}
	}()

	w.logger.Debug("Flushing worker")

	// Generate bulk request.
	req := opensearchapi.BulkReq{}
	if w.bi.config.Index != "" {
		req.Index = w.bi.GetClient().IndexWriteAlias(w.bi.config.Index)
	}
	if w.bi.config.Params != nil {
		req.Params = w.bi.config.Params
	}
	req.Header = opensearch.NewRequestHeaders(ctx)
	req.Body = w.buf

	// Perform bulk request.
	resp, err := w.bi.GetClient().Bulk(ctx, req)
	if err != nil {
		for _, sub := range opensearchapi.Errors(err) {
			switch e := sub.(type) {
			case *opensearchapi.PartialBulkError:
				w.processPartialErrors(e)
			default:
				return fmt.Errorf("bulk request: %w", err)
			}
		}
	}
	w.logger.Debug("Worker flushed",
		slog.Duration("took", time.Duration(resp.Took)),
		slog.Int("count", len(resp.Items)),
	)

	return nil
}

func (w *worker) processPartialErrors(errs *opensearchapi.PartialBulkError) {
	w.logger.Warn(
		"Some items failed to be indexed",
		slog.Int("successful", errs.SucceededCount),
		slog.Int("failed", len(errs.FailedItems)),
	)
	for item := range slices.Values(errs.FailedItems) {
		if item.Error != nil {
			w.logger.Warn("Item index failed.", slog.String("id", *item.ID), slog.Any("error", item.Error))
		}
	}
}
