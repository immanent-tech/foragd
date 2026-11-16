package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/immanent-tech/go-base/logging"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/ollama"
	"github.com/immanent-tech/foragd/providers/opensearch"
	osbulk "github.com/immanent-tech/foragd/providers/opensearch/bulk"
	"github.com/immanent-tech/foragd/service"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New())

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	elasticSvc, err := service.LoadElasticService()
	if err != nil {
		panic(err)
	}

	osc, err := opensearch.GetClient()
	if err != nil {
		slogctx.Warn(ctx, "Failed to load opensearch client", slog.Any("error", err))
		// return nil, fmt.Errorf("load opensearch client: %w", err)
	}
	indexer, err := osbulk.New()
	if err != nil {
		slogctx.Warn(ctx, "Failed to load bulk indexer", slog.Any("error", err))
	}

	httpClient := service.NewHTTPClient()
	ollamaSvc, err := ollama.NewClient()
	if err != nil {
		panic(fmt.Errorf("load ollama client: %w", err))
	}
	itemsSvc, err := service.LoadItemsService(ctx, osc, indexer, httpClient)
	if err != nil {
		panic(fmt.Errorf("load items service: %w", err))
	}
	feedsSvc, err := service.LoadFeedsService(ctx, osc, indexer, httpClient, ollamaSvc, itemsSvc)
	if err != nil {
		panic(fmt.Errorf("load feeds service: %w", err))
	}

	slogctx.FromCtx(ctx).Info("Get all feeds.")
	feeds, err := elastic.SearchAll[*models.Feed](
		ctx,
		elasticSvc.GetIndexRO(service.FeedsIndex),
		query.Bool(
			query.WithBoolShouldMinimumMatch(1),
			query.Should(
				query.Term("categories.raw", "Uncategorized"),
				query.Bool(
					query.MustNot(
						query.Exists("categories"),
					),
				),
			),
		),
		5000,
	)
	if err != nil {
		panic(err)
	}

	for feed := range slices.Values(feeds) {
		ctx := slogctx.With(ctx, "feed_id", feed.GetID())
		ctx = slogctx.With(ctx, "feed_title", feed.GetTitle())

		slogctx.FromCtx(ctx).Info("Classifying feed.")
		items, _, err := itemsSvc.QueryItems(ctx,
			query.Term("feed_id", feed.GetID()),
			10,
			nil,
			nil,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not fetch items for feed",
				slog.Any("error", err))
		}

		feed.Items = items

		if err := feedsSvc.ClassifyFeed(ctx, feed); err != nil {
			slogctx.FromCtx(ctx).Error("Could not classify feed. Leaving uncategorized.",
				slog.Any("error", err))
			feed.Categories = []models.Category{"Uncategorized"}
		}

		slogctx.FromCtx(ctx).Info("Feed classified.",
			slog.String("categories", strings.Join(feed.Categories, ",")))

		if err := feedsSvc.UpdateFeed(ctx, feed); err != nil {
			slogctx.FromCtx(ctx).Warn("Update feed failed.",
				slog.Any("error", err))
		}

		bulk.Flush(ctx)
	}
}
