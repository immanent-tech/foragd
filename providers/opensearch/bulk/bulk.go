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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"slices"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v5/opensearchutil"
	slogctx "github.com/veqryn/slog-context"
)

const (
	defaultFlushBytes    = 5e+6
	defaultFlushInterval = 30 * time.Second
)

// ErrIndexerClosed is returned when Flush is called on a closed indexer.
var ErrIndexerClosed = errors.New("bulk indexer is closed")

type Config struct {
	opensearchutil.BulkIndexerConfig
}

// Indexer represents an OpenSearch bulk indexer instance.
type Indexer struct {
	opensearchutil.BulkIndexer

	logger *slog.Logger
}

// New creates a new bulk indexer instance with the given options.
func New(options ...IndexerOption) (*Indexer, error) {
	// Set up config.
	cfg := &Config{
		NumWorkers:    runtime.NumCPU(),
		FlushBytes:    defaultFlushBytes,
		FlushInterval: defaultFlushInterval,
		RetryPolicy: opensearchutil.RetryPolicyFunc(
			func(attempt int, err *opensearchutil.BulkError) (time.Duration, bool) {
				if errors.Is(err, opensearchutil.ErrPermanent) || attempt >= 5 {
					return 0, false
				}
				return 100 * time.Millisecond << (attempt - 1), true // 100ms, 200ms, 400ms, ...
			},
		),
	}
	for option := range slices.Values(options) {
		option(cfg)
	}

	indexer, err := opensearchutil.NewBulkIndexer(cfg.BulkIndexerConfig)
	if err != nil {
		return nil, fmt.Errorf("create bulk indexer: %w", err)
	}

	return &Indexer{BulkIndexer: indexer}, nil
}

// IndexerOption is a functional option for configuring the bulk indexer.
type IndexerOption func(*Config)

// WithNumWorkers option sets the number of parallel bulk indexer workers.
func WithNumWorkers(workers int) IndexerOption {
	return func(c *Config) {
		c.NumWorkers = workers
	}
}

// WithFlushBytes option sets the number of bytes after which a bulk indexer worker will flush its queue. Defaults to
// [defaultFlushBytes] if this option is not used.
func WithFlushBytes(value int) IndexerOption {
	return func(c *Config) {
		c.FlushBytes = value
	}
}

// WithFlushInterval option sets the duration after which a bulk indexer worker will flush its queue. Defaults to
// [defaultFlushInterval] if this option is not used.
func WithFlushInterval(interval time.Duration) IndexerOption {
	return func(c *Config) {
		c.FlushInterval = interval
	}
}

// WithIndex option sets the index to which all actions are applied.
func WithIndex(index string) IndexerOption {
	return func(c *Config) {
		c.Index = index
	}
}

// WithOnError option sets a callback function to be called on errors.
func WithOnError(onError func(context.Context, error)) IndexerOption {
	return func(c *Config) {
		c.OnUnhandledError = onError
	}
}

// WithOnWarning option sets a callback function to be called on warnings.
func WithOnWarning(onWarning func(context.Context, error)) IndexerOption {
	return func(c *Config) {
		c.OnWarning = onWarning
	}
}

// WithOnFlushStart option sets a callback function to be called when a flush starts.
func WithOnFlushStart(onFlushStart func(context.Context) context.Context) IndexerOption {
	return func(c *Config) {
		c.OnFlushStart = onFlushStart
	}
}

// WithOnFlushEnd option sets a callback function to be called when a flush ends.
func WithOnFlushEnd(onFlushEnd func(context.Context)) IndexerOption {
	return func(c *Config) {
		c.OnFlushEnd = onFlushEnd
	}
}

// Add adds an item to the indexer.
func (bi *Indexer) Add(ctx context.Context, action Action) error {
	if err := ctx.Err(); err != nil {
		slogctx.Warn(ctx, "context done",
			slog.Any("cause", context.Cause(ctx)),
			slog.Any("error", err),
		)
		return fmt.Errorf("add bulk action: %w", err)
	}
	if err := action.encode(); err != nil {
		return fmt.Errorf("encode item: %w", err)
	}
	if err := bi.BulkIndexer.Add(ctx, action.BulkIndexerItem); err != nil {
		return fmt.Errorf("add action: %w", err)
	}
	return nil
}

type BulkResponse struct {
	*opensearchapi.BulkResp
}
