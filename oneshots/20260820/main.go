package main

import (
	"context"
	"log/slog"
	"net/url"
	"slices"

	"github.com/immanent-tech/go-base/logging"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/google/youtube"
	"github.com/immanent-tech/foragd/service"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New())

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	slogctx.FromCtx(ctx).Info("Get all feeds.")
	feeds, err := elastic.SearchAll[*models.Feed](
		ctx,
		schema.FeedsIndexRO(),
		query.Term("domain.raw", "www.youtube.com"),
		5000)
	if err != nil {
		panic(err)
	}

	for feed := range slices.Values(feeds) {
		ctx := slogctx.With(ctx, "feed_id", feed.GetID())
		ctx = slogctx.With(ctx, "feed_title", feed.GetTitle())

		slogctx.Info(ctx, "Updating feed.")

		feedURL, err := url.Parse(feed.GetSourceURLs()[0])
		if err != nil {
			slogctx.Error(ctx, "Could not extract feed URL.", slog.Any("error", err))
			continue
		}
		channelID, found := feedURL.Query()["channel_id"]
		if !found {
			slogctx.Error(ctx, "Could not determine channel ID from URL.", slog.String("url", feedURL.String()))
			continue
		}

		sourceData := &models.Feed_SourceData{}
		sourceData.FromYoutubeFeedData(models.YoutubeFeedData{
			ID:   channelID[0],
			Type: youtube.TypeChannel,
		})

		if err := service.UpdateFeed(ctx, feed.GetID(), map[string]any{
			"source_data": sourceData,
			"source_type": models.SourceTypeYoutube,
		}); err != nil {
			slogctx.FromCtx(ctx).Warn("Update feed failed.",
				slog.Any("error", err))
		}
	}
}
