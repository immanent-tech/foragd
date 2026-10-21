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

	slogctx.FromCtx(ctx).Info("Get all feeds.")
	feeds, err := elastic.SearchAll[*models.Feed](
		ctx,
		elasticSvc.GetIndexRO(service.FeedsIndex),
		query.MatchAll(),
		5000)
	if err != nil {
		panic(err)
	}

	for feed := range slices.Values(feeds) {
		ctx := slogctx.With(ctx, "feed_id", feed.GetID())
		ctx = slogctx.With(ctx, "feed_title", feed.GetTitle())

		originalCategories := feed.Categories
		slices.Sort(originalCategories)
		newCategories := slices.CompactFunc(originalCategories, func(a, b models.Category) bool {
			return strings.EqualFold(a, b)
		})
		if len(originalCategories) > len(newCategories) {
			slogctx.Info(ctx, "Duplicate feed categories removed.",
				slog.String("feed_id", feed.GetID()),
				slog.String("before", strings.Join(originalCategories, ",")),
				slog.String("after", strings.Join(newCategories, ",")),
			)
			feed.Categories = newCategories
			if err := feedSvc.UpdateFeed(ctx, feed); err != nil {
				slogctx.FromCtx(ctx).Warn("Update feed failed.",
					slog.Any("error", err))
			}
		}
	}

	bulk.Flush(ctx)
}
