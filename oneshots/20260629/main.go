package main

import (
	"context"
	"log/slog"
	"net/url"
	"slices"

	"github.com/goforj/godump"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New(logging.Options{LogLevel: "debug", NoLogFile: true}))

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	slogctx.FromCtx(ctx).Info("Get all feeds.")
	feeds, err := elastic.SearchAll[*models.Feed](ctx, schema.FeedsIndexRO(), query.MatchAll(), 5000)
	if err != nil {
		panic(err)
	}

	slogctx.FromCtx(ctx).Info("Update feeds to add domains.")
	for feed := range slices.Values(feeds) {
		if feed.Domain != "" {
			continue
		}
		link, err := url.Parse(feed.GetLink())
		if err != nil {
			slogctx.Warn(ctx, "Failed to parse feed link.",
				slog.String("link", feed.GetLink()),
				slog.Any("error", err))
			continue
		}
		feed.Domain = link.Hostname()
		slogctx.Info(ctx, "Added domain to feed.",
			slog.String("feed_id", feed.GetID()),
			slog.String("feed_title", feed.GetTitle()),
			slog.String("domain", feed.Domain))
	}

	results, err := elastic.BulkUpdate(ctx, schema.FeedsIndexRW(), feeds...)
	if err != nil {
		godump.Dump(results)
		panic(err)
	}
}
