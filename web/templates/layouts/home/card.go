// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/display/card"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
)

type FeedCard struct {
	Card
	UnreadCount int
}

type ItemCard struct {
	Card
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
	route := CurrentRouteFromCtx(ctx)
	route.Filters.Pagination = pagination
	action := route.AsAction()
	action.SetAttributes(templ.Attributes{
		"hx-trigger":   "intersect once",
		"hx-swap":      "afterend",
		"hx-push-url":  "false",
		"hx-indicator": "#content-loading",
	})
	c.AddAttributes(action.Attributes())
}

func BuildFeedsLayout(ctx context.Context, pagination models.Pagination, subscriptions models.Subscriptions) templates.Layout {
	cards := make([]templ.Component, 0, len(subscriptions))
	// Build feed cards.
	for idx, subscription := range subscriptions {
		card := newFeedCard(ctx, subscription)
		if idx == len(subscriptions)-1 && len(subscriptions) == CurrentRouteFromCtx(ctx).Filters.Count {
			card.addPagination(ctx, pagination)
		}

		cards = append(cards, card.Show())
	}
	return &HomeLayout{
		title:   "Feeds",
		content: cards,
		footer: BuildListFooter(ctx,
			subscriptions.GetCategoryCounts(),
			addSubscriptionAction(),
			importAction(),
			markAllFeedsAction(CurrentRouteFromCtx(ctx).Filters.View),
		).Show(),
	}
}

func BuildItemsLayout(ctx context.Context, pagination models.Pagination, items models.Items) templates.Layout {
	cards := make([]templ.Component, 0, len(items))
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard := newItemCard(ctx, item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == CurrentRouteFromCtx(ctx).Filters.Count {
			itemCard.addPagination(ctx, pagination)
		}
		// Append the card to the list of cards.
		cards = append(cards, itemCard.Show())
	}
	// Return the home items layout.
	return &HomeLayout{
		title:   "Items",
		content: cards,
		footer: BuildListFooter(ctx,
			items.GetCategoryCounts(),
			addSubscriptionAction(),
			importAction(),
			markAllItemsAction(items.GetFeedIDs(), CurrentRouteFromCtx(ctx).Filters.View),
		).Show(),
	}
}
