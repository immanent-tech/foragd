package main

import (
	"context"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/goforj/godump"
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
		query.Bool(
			query.Filter(
				query.TermWithEmpty("source_urls", ""),
			),
		),
		5000)
	if err != nil {
		panic(err)
	}

	failed := make([]models.FeedID, 0)

	for feed := range slices.Values(feeds) {
		ctx := slogctx.With(ctx, "feed_id", feed.GetID())
		ctx = slogctx.With(ctx, "feed_title", feed.GetTitle())

		var (
			newFeed *models.Feed
			err     error
		)

		if feed.Domain == "news.google.com" {
			link, err := url.Parse(feed.GetLink())
			if err != nil {
				slogctx.Warn(ctx,
					"Could not parse Google news URL",
					slog.String("link", feed.GetLink()),
					slog.Any("error", err),
				)
				continue
			}
			link.Path = "/rss" + link.Path
			feed.SourceURLs = []models.URL{link.String()}
			newFeed = feed
		} else {
			newFeed, err = feedSvc.FetchFeed(ctx, feed.GetLink(), service.ApplyExistingID(feed.GetID()))
			if err != nil {
				slogctx.Warn(ctx, "Could not fetch feed", slog.Any("error", err))
				failed = append(failed, feed.GetID())
				continue
			}
		}

		if err := newFeed.Validate(); err != nil {
			slogctx.Error(ctx, "Feed invalid!", slog.Any("error", err))
			failed = append(failed, feed.GetID())
			continue
		}
		if err := feedSvc.UpdateFeed(ctx, newFeed); err != nil {
			slogctx.Warn(ctx, "Update feed failed.",
				slog.Any("error", err))
			failed = append(failed, feed.GetID())
			continue
		}
		bulk.Flush(ctx)
		slogctx.Info(ctx, "Feed URLs updated.",
			slog.String("source_urls", strings.Join(newFeed.GetSourceURLs(), ",")),
			slog.String("link", newFeed.GetLink()),
		)
	}

	godump.Dump(failed)
}
