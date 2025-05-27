// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"log/slog"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/display/card"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

type FeedCard struct {
	Card
	UnreadCount int
}

type ItemCard struct {
	Card
}

type Article struct {
	models.SourceWithContent
}

type cardActions []templ.Component

// Card is a display component that shows a DaisyUI Card for the given data.
type Card struct {
	id          string
	menuActions cardActions
	models.Source
	*card.Props
}

// AddPagination adds htmx attributes for triggering pagination to a card.
func (c *Card) addPagination(ctx context.Context, pagination models.Pagination) {
	view := models.ViewFromCtx(ctx)
	c.AddAttributes(templ.Attributes{
		"hx-get":         view.String() + "&pagination=" + pagination,
		"hx-trigger":     "intersect once",
		"hx-swap":        "afterend",
		"hx-replace-url": "false",
		"hx-indicator":   "#content-loading",
	})
}

func BuildFeedsLayout(ctx context.Context, pagination models.Pagination, subscriptions models.Subscriptions) *templates.Body {
	cards := GenerateFeedCards(ctx, subscriptions, pagination)
	// Generate backlink.
	return templates.NewBody(
		templ.Join(cards...),
		templates.WithBodyHeader(
			partials.Header(
				partials.DefaultHeaderStart(),
				partials.DefaultHeaderCenter(),
				partials.DefaultHeaderEnd(),
			),
		),
		templates.WithBodyFooter(
			partials.Footer(
				partials.UpdateBacklink(),
				UpdateFilters(subscriptions.GetCategoryCounts()),
				UpdateSorting(),
				UpdateActions(
					AddSubscriptionAction(),
					ImportAction(),
					MarkAllFeedsAction(ctx),
				),
			),
		),
	)
}

func GenerateFeedCards(ctx context.Context, subscriptions models.Subscriptions, pagination models.Pagination) []templ.Component {
	view := models.ViewFromCtx(ctx)
	cards := make([]templ.Component, 0, len(subscriptions))
	for idx, subscription := range subscriptions {
		card := newFeedCard(ctx, subscription)
		if idx == len(subscriptions)-1 && len(subscriptions) == view.Filters.Count {
			card.addPagination(ctx, pagination)
		}

		cards = append(cards, card.Show())
	}
	return cards
}

func BuildItemsLayout(ctx context.Context, pagination models.Pagination, items models.Items) *templates.Body {
	cards := GenerateItemCards(ctx, items, pagination)
	// Return the home items layout.
	return templates.NewBody(
		templ.Join(cards...),
		templates.WithBodyHeader(
			partials.Header(
				partials.DefaultHeaderStart(),
				partials.DefaultHeaderCenter(),
				partials.DefaultHeaderEnd(),
			),
		),
		templates.WithBodyFooter(
			partials.Footer(
				partials.UpdateBacklink(),
				UpdateFilters(items.GetCategoryCounts()),
				UpdateSorting(),
				UpdateActions(
					AddSubscriptionAction(),
					ImportAction(),
					MarkAllItemsAction(ctx, items.GetFeedIDs()),
				),
			),
		),
	)
}

func GenerateItemCards(ctx context.Context, items models.Items, pagination models.Pagination) []templ.Component {
	view := models.ViewFromCtx(ctx)
	cards := make([]templ.Component, 0, len(items))
	for idx, item := range items {
		slogctx.FromCtx(ctx).Debug("displaying item", slog.String("item_id", item.GetID()), slog.Bool("state", item.IsUnread()))
		// Create a card for this item.
		itemCard := newItemCard(ctx, item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == view.Filters.Count && pagination != "" {
			itemCard.addPagination(ctx, pagination)
		}
		// Append the card to the list of cards.
		cards = append(cards, itemCard.Show())
	}
	return cards
}

func GenerateArticle(item *models.Item) templ.Component {
	article := &Article{SourceWithContent: item}
	return article.Show()
}
