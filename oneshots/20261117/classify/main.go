/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"runtime"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-base/logging"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/ollama"
	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/providers/opensearch/search"
	"github.com/immanent-tech/foragd/schema"
)

func main() {
	ctx, cancelFunc := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelFunc()
	ctx = slogctx.NewCtx(ctx, logging.New())

	osc, err := opensearch.GetClient()
	if err != nil {
		panic(fmt.Errorf("load opensearch client: %w", err))
	}

	indexer, err := bulk.New(osc)
	if err != nil {
		slogctx.Warn(ctx, "Failed to load bulk indexer", slog.Any("error", err))
	}

	ollamaSvc, err := ollama.NewClient()
	if err != nil {
		panic(fmt.Errorf("load ollama client: %w", err))
	}

	paginationSize := 2500
	var searchAfter []opensearchapi.FieldValue

	for {
		body := &opensearchapi.SearchBody{}

		loopReq := &opensearchapi.SearchReq{
			Indices: []string{osc.IndexReadAlias(schema.ItemsIndex)},
			Header:  opensearch.NewRequestHeaders(ctx),
			Body:    body,
		}
		search.WithQuery(
			search.BoolQuery(
				search.WithBoolFilter(
					search.ExistsQuery("content"),
				),
				search.WithBoolMustNot(
					search.ExistsQuery("content_semantic"),
				),
			),
		)(loopReq)
		search.WithSearchAfter(searchAfter)(loopReq)
		search.WithSearchParameters(search.WithSortParams("_doc"))(loopReq)
		search.WithSize(paginationSize)(loopReq)

		loopResp, err := osc.Search(ctx, loopReq)
		if err != nil {
			panic(opensearch.HandleError("search", loopResp, err))
		}

		// Stop looping if we've hit the end of the results.
		if len(loopResp.Hits.Hits) < paginationSize {
			break
		}

		slogctx.Info(ctx, "Pagination done.",
			slog.Int("hits", len(loopResp.Hits.Hits)),
			slog.Duration("took", time.Duration(loopResp.Took)),
		)

		// Update search after.
		searchAfter = loopResp.Hits.Hits[len(loopResp.Hits.Hits)-1].Sort

		// Extract hits.
		hits := make(models.Items, 0, len(loopResp.Hits.Hits))
		for hit := range slices.Values(loopResp.Hits.Hits) {
			source, err := opensearch.ExtractSource[*models.Item](hit.Source)
			if err != nil {
				slogctx.Warn(ctx, "Could not extract item.", slog.Any("error", err))
				continue
			}
			hits = append(hits, source)
		}

		slogctx.Info(ctx, "Items extracted.",
			slog.Int("count", len(hits)),
		)

		var (
			g               errgroup.Group
			classifiedTotal atomic.Int32
		)
		g.SetLimit(runtime.GOMAXPROCS(0))
		errs := make([]error, 0)
		start := time.Now()

		for item := range slices.Values(hits) {
			g.Go(func() error {
				if item.GetContent() != "" {
					resp, err := ollamaSvc.EmbedBatch(ctx, item.GetContent())
					if err != nil {
						errs = append(errs, fmt.Errorf("generate embeddings %s: %w", item.GetID(), err))
						return nil
					}
					if err := indexer.Add(
						ctx,
						bulk.NewAction(
							bulk.ActionUpdate,
							bulk.ToIndex(osc.IndexWriteAlias(schema.ItemsIndex)),
							bulk.WithID(item.GetID()),
							bulk.WithDoc(map[string]any{
								"doc": map[string]any{
									"content_semantic": resp[0],
								},
							}),
						),
					); err != nil {
						errs = append(errs, fmt.Errorf("update item %s: %w", item.GetID(), err))
						return nil
					}
					classifiedTotal.Add(1)
				}
				return nil
			})
		}

		_ = g.Wait()
		if len(errs) > 0 {
			slogctx.Warn(
				ctx,
				"Some items failed to be enriched.",
				slog.Int("count", int(len(errs))),
				slog.Any("errors", errors.Join(errs...)),
			)
		}

		slogctx.Info(ctx, "Clasified items",
			slog.Int("count", int(classifiedTotal.Load())),
			slog.Duration("took", time.Since(start)))
		if err := indexer.Flush(ctx); err != nil {
			slogctx.Warn(ctx, "Flush indexer failed", slog.Any("error", err))
		}
	}

	<-ctx.Done()
	cancelFunc()

	shutdownCtx, cancel := context.WithTimeoutCause(
		context.Background(),
		30*time.Second,
		errors.New("graceful shutdown timeout"),
	)
	shutdownCtx = slogctx.NewCtx(shutdownCtx, slogctx.FromCtx(ctx))
	defer cancel()

	if err := ollamaSvc.Close(); err != nil {
		slogctx.Warn(shutdownCtx, "Could not shutdown ollama cleanly", slog.Any("error", err))
	}
	if err := indexer.Flush(shutdownCtx); err != nil {
		slogctx.Warn(shutdownCtx, "Flush indexer failed", slog.Any("error", err))
	}
	if err := indexer.Close(shutdownCtx); err != nil {
		slogctx.Warn(shutdownCtx, "Could not shutdown indexer cleanly", slog.Any("error", err))
	}
	if err := osc.Close(); err != nil {
		slogctx.Warn(shutdownCtx, "Could not shutdown opensearch client cleanly", slog.Any("error", err))
	}
}
