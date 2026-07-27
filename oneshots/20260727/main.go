// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/immanent-tech/go-base/logging"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/vector"
	"github.com/immanent-tech/foragd/providers/ollama"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New())

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	slogctx.FromCtx(ctx).Info("Get all items.")
	items, err := elastic.SearchAll[*models.Item](ctx, schema.ItemsIndexRO(), query.Exists("content"), 5000)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				bulk.LogStats(ctx)
				time.Sleep(5 * time.Second)
			}
		}
	}()

	slogctx.FromCtx(ctx).Info("Update items to add vectors.")
	for item := range slices.Values(items) {
		// Ignore items without content.
		if item.GetContent() == "" {
			continue
		}
		// Ignore items with vectors.
		if item.ContentVector != nil {
			continue
		}
		chunks := vector.ChunkBytes([]byte(*item.Content), 2000, 200)
		vectors, err := ollama.EmbedChunks(chunks...)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not embed content.", slog.Any("error", err))
		} else {
			item.ContentVector = vectors[0]
			if err := bulk.AddAction(ctx,
				bulk.NewAction(item,
					bulk.AsOperation[models.ItemID](bulk.OpIndex),
					bulk.ToIndex[models.ItemID](schema.ItemsIndexRW()),
				),
			); err != nil {
				slogctx.FromCtx(ctx).Error("Unable to bulk update item.",
					slog.String("item_id", item.GetID()),
					slog.Any("error", err),
				)
			}
		}
	}

}
