// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package bulk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elastic/go-elasticsearch/v9/esapi"
	"github.com/elastic/go-elasticsearch/v9/esutil"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/providers/elastic"
)

const (
	defaultRetryCount = 3
)

const (
	OpIndex  Operation = "index"
	OpCreate Operation = "create"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
)

// Operation is the type of bulk operation for a given action.
type Operation string

var (
	ErrInitIndexer = errors.New("indexer not initialised")
)

var (
	getIndexer  func() (*Indexer, error)
	indexer     *Indexer
	initialized atomic.Bool
)

// Indexer represents a bulk indexer instance. The indexer instance is generally created once when needed and then
// reused throughout the application life cycle.
type Indexer struct {
	esutil.BulkIndexer
}

// NewIndexer creates a new indexer. It will initialize the indexer once, if not done already, then return the instance.
func NewIndexer(ctx context.Context, options ...IndexerOption) (*Indexer, error) {
	if err := initIndexer(ctx, options...); err != nil {
		return nil, fmt.Errorf("create indexer: %w", err)
	}

	var err error
	indexer, err = getIndexer()
	if err != nil {
		return nil, fmt.Errorf("get indexer: %w", err)
	}

	return indexer, nil
}

// Shutdown gracefully closes the indexer (if it has been initialized/created).
func Shutdown(ctx context.Context) error {
	if initialized.Load() {
		if err := indexer.Close(ctx); err != nil {
			return fmt.Errorf("close indexer: %w", err)
		}
	}
	initialized.Store(false)
	slogctx.FromCtx(ctx).Debug("Bulk indexer shutdown.",
		slog.Time("stop_time", time.Now().UTC()))
	return nil
}

func initIndexer(ctx context.Context, options ...IndexerOption) error {
	if !initialized.Load() {
		api, err := elastic.GetAPI()
		if err != nil {
			return fmt.Errorf("get elastic api: %w", err)
		}
		setupIndexer(ctx, api, options...)
	}
	return nil
}

func setupIndexer(ctx context.Context, api esapi.Transport, options ...IndexerOption) {
	getIndexer = sync.OnceValues(func() (*Indexer, error) {
		initialized.Store(true)
		opts := &esutil.BulkIndexerConfig{
			Client: api,
			OnError: func(ctx context.Context, err error) {
				slogctx.FromCtx(ctx).Error("Bulk operation failed.",
					slog.Any("error", err))
			},
		}
		for option := range slices.Values(options) {
			option(opts)
		}

		// If no flush interval specified, set a default.
		if opts.FlushInterval == 0 {
			opts.FlushInterval = time.Minute
			opts.FlushJitter = 5 * time.Second
		}

		indexer, err := esutil.NewBulkIndexer(*opts)
		if err != nil {
			return nil, fmt.Errorf("init indexer: %w", err)
		}

		slogctx.FromCtx(ctx).Debug("Bulk indexer created.",
			slog.Time("start_time", time.Now().UTC()))

		return &Indexer{BulkIndexer: indexer}, nil
	})
}

// Flush will flush all pending bulk actions, forcing Elasticsearch to process them.
func Flush(ctx context.Context) error {
	if _, err := NewIndexer(context.Background()); err != nil {
		return fmt.Errorf("%w: %w", ErrInitIndexer, err)
	}
	if err := indexer.Flush(ctx); err != nil {
		return fmt.Errorf("flush indexer: %w", err)
	}
	return nil
}

// IndexerOption is a functional option to apply to an indexer instance.
type IndexerOption func(*esutil.BulkIndexerConfig)

// WithFlushInterval defines the interval (with jitter) after which any pending bulk operations will be flushed.
func WithFlushInterval(interval, jitter time.Duration) IndexerOption {
	return func(bic *esutil.BulkIndexerConfig) {
		bic.FlushInterval = interval
		bic.FlushJitter = jitter
	}
}

// IndexDocuments wraps AddAction to bulk index the given documents to the given index.
func IndexDocuments[T ~string, O Document[T]](
	ctx context.Context,
	index string,
	documents ...O,
) error {
	if _, err := NewIndexer(context.Background()); err != nil {
		return fmt.Errorf("%w: %w", ErrInitIndexer, err)
	}

	actions := make([]*Action[T], 0, len(documents))

	for document := range slices.Values(documents) {
		actions = append(actions, NewAction(document, ToIndex[T](index), AsOperation[T](OpIndex)))
	}

	for action := range slices.Values(actions) {
		item, err := action.marshalItem()
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable to marshal document to item.",
				slog.Any("error", err))
		}

		if err := indexer.Add(ctx, *item); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to add document.",
				slog.Any("error", err))
		}
	}

	if err := indexer.Flush(ctx); err != nil {
		return fmt.Errorf("flush indexer: %w", err)
	}

	return nil
}

// AddAction sends the given actions to the indexer.
func AddAction[T ~string](
	ctx context.Context,
	actions ...*Action[T],
) error {
	if _, err := NewIndexer(context.Background()); err != nil {
		return fmt.Errorf("%w: %w", ErrInitIndexer, err)
	}

	for action := range slices.Values(actions) {
		item, err := action.marshalItem()
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable to marshal document to item.",
				slog.Any("error", err))
		}

		if err := indexer.Add(ctx, *item); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to add document.",
				slog.Any("error", err))
		}
	}

	return nil
}

// Action represents a single bulk action.
type Action[T ~string] struct {
	doc     Document[T]
	index   string
	retries int
	op      Operation
}

// NewAction creates a new bulk action for the given document, with the given options.
func NewAction[T ~string](doc Document[T], options ...ActionOption[T]) *Action[T] {
	item := &Action[T]{
		doc:     doc,
		retries: defaultRetryCount,
	}

	for option := range slices.Values(options) {
		option(item)
	}

	return item
}

func (a *Action[T]) marshalItem() (*esutil.BulkIndexerItem, error) {
	rd, err := docToReader(a.doc)
	if err != nil {
		return nil, fmt.Errorf("create doc reader: %w", err)
	}

	return &esutil.BulkIndexerItem{
		DocumentID:      string(a.doc.GetID()),
		Body:            rd,
		Index:           a.index,
		Action:          string(a.op),
		RetryOnConflict: &a.retries,
		OnFailure: func(ctx context.Context, _ esutil.BulkIndexerItem, biri esutil.BulkIndexerResponseItem, err error) {
			slogctx.FromCtx(ctx).Warn("Failed to bulk index item.",
				slog.String("id", biri.DocumentID),
				slog.Any("error", err),
			)
		},
	}, nil
}

// ActionOption is a functional option applied to an action.
type ActionOption[T ~string] func(*Action[T])

func AsOperation[T ~string](op Operation) ActionOption[T] {
	return func(a *Action[T]) {
		a.op = op
	}
}

// ToIndex option defines the index to operate on.
func ToIndex[T ~string](index string) ActionOption[T] {
	return func(i *Action[T]) {
		i.index = index
	}
}

// WithRetries option defines the number of retries if this action fails. Set to zero to disable retries.
func WithRetries[T ~string](retries int) ActionOption[T] {
	return func(i *Action[T]) {
		i.retries = retries
	}
}

// Document represents any type of document stored in any index.
type Document[T ~string] interface {
	GetID() T
}

// PartialDocument represents partial data for a given document.
type PartialDocument struct {
	Parts map[string]any `json:"doc"`
	ID    string         `json:"-"`
}

func (d *PartialDocument) GetID() string {
	return d.ID
}

func docToReader[T any](doc T) (io.ReadSeeker, error) {
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal document: %w", err)
	}
	return bytes.NewReader(data), nil
}
