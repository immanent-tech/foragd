package main

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/immanent-tech/go-base/logging"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
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

	feedSvc, err := service.LoadFeedService()
	if err != nil {
		panic(err)
	}

	itemSvc, err := service.LoadItemService()
	if err != nil {
		panic(err)
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
		items, _, err := itemSvc.QueryItems(ctx,
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

		if err := feedSvc.ClassifyFeed(ctx, feed); err != nil {
			slogctx.FromCtx(ctx).Error("Could not classify feed. Leaving uncategorized.",
				slog.Any("error", err))
			feed.Categories = []models.Category{"Uncategorized"}
		}

		slogctx.FromCtx(ctx).Info("Feed classified.",
			slog.String("categories", strings.Join(feed.Categories, ",")))

		if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
			slogctx.FromCtx(ctx).Warn("Update feed failed.",
				slog.Any("error", err))
		}

		bulk.Flush(ctx)
	}
}
