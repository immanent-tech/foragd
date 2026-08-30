package main

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/immanent-tech/go-base/logging"
	"github.com/immanent-tech/go-base/pkg/textx"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/ollama"
	"github.com/immanent-tech/foragd/service"
)

func main() {
	ctx := slogctx.NewCtx(context.Background(), logging.New())

	if err := elastic.Connect(); err != nil {
		panic(err)
	}

	slogctx.FromCtx(ctx).Info("Get all feeds.")
	feeds, err := elastic.SearchAll[*models.Feed](ctx, schema.FeedsIndexRO(), query.MatchAll(), 5000)
	if err != nil {
		panic(err)
	}

	for feed := range slices.Values(feeds) {
		ctx := slogctx.With(ctx, "feed_id", feed.GetID())
		ctx = slogctx.With(ctx, "feed_title", feed.GetTitle())

		slogctx.FromCtx(ctx).Info("Classifying feed.")
		items, _, err := service.SearchItems(ctx,
			query.Term("feed_id", feed.GetID()),
			10,
			nil,
			nil,
		)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not fetch items for feed",
				slog.Any("error", err))
		}

		var (
			classificationText strings.Builder
		)

		// Append together all content for classification.
		if feed.GetDescription() != "" {
			classificationText.WriteString(feed.GetDescription())
			classificationText.WriteRune('\n')
		}
		for item := range slices.Values(items) {
			switch {
			case item.GetContent() != "":
				classificationText.WriteString(item.GetContent())
				classificationText.WriteRune('\n')
			case item.GetDescription() != "":
				classificationText.WriteString(item.GetDescription())
				classificationText.WriteRune('\n')
			}
		}

		if textx.CountWords(classificationText.String()) < 20 {
			// Ignore feed when there is too little content for good processing.
			slogctx.FromCtx(ctx).Warn("Not enough content to classify feed. Assigning 'Uncategorized'.")
			feed.Categories = []models.Category{"Uncategorized"}
		} else {
			// Classify from IAB Tier 1 Categories.
			slogctx.FromCtx(ctx).Info("Assigning categories to feed based on current item content.")

			categories, err := ollama.Classify(classificationText.String(), feed.GetLink(), nil, nil, 0.8)
			if err != nil {
				slogctx.FromCtx(ctx).Error("Could not classify feed. Leaving uncategorized.",
					slog.Any("error", err))
				feed.Categories = []models.Category{"Uncategorized"}
			}
			if len(categories) == 0 {
				slogctx.FromCtx(ctx).Warn("No classified categories. Leaving uncategorized.")
				feed.Categories = []models.Category{"Uncategorized"}
			} else {
				feed.Categories = make([]models.Category, 0, len(categories))
				for category := range slices.Values(categories) {
					feed.Categories = append(feed.Categories, category.Label)
				}
			}
		}

		slogctx.FromCtx(ctx).Info("Feed classified.",
			slog.String("categories", strings.Join(feed.Categories, ",")))

		if err := service.UpdateFeed(ctx, feed.GetID(), map[string]any{
			"categories": feed.Categories,
		}); err != nil {
			slogctx.FromCtx(ctx).Warn("Update feed failed.",
				slog.Any("error", err))
		}
	}
}
