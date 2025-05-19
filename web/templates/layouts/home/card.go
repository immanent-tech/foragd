// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/display/card"

	"github.com/joshuar/go-feed-me/models"
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
	route := CurrentViewFromCtx(ctx)
	action := route.AsAction()
	action.Params.Add(models.ParamPagination, pagination)
	action.SetAttributes(templ.Attributes{
		"hx-trigger":   "intersect once",
		"hx-swap":      "afterend",
		"hx-push-url":  "false",
		"hx-indicator": "#content-loading",
	})
	c.AddAttributes(action.Attributes())
}

func BuildFeedsLayout(ctx context.Context, pagination models.Pagination, subscriptions models.Subscriptions) *HomeLayout {
	cards := make([]templ.Component, 0, len(subscriptions))
	// Build feed cards.
	for idx, subscription := range subscriptions {
		card := newFeedCard(ctx, subscription)
		if idx == len(subscriptions)-1 && len(subscriptions) == CurrentViewFromCtx(ctx).Filters.Count {
			card.addPagination(ctx, pagination)
		}

		cards = append(cards, card.Show())
	}
	return &HomeLayout{
		Title:   "Feeds",
		Content: cards,
		Footer: BuildListFooter(ctx,
			models.FeedsRoute,
			subscriptions.GetCategoryCounts(),
			addSubscriptionAction(),
			importAction(),
			markAllFeedsAction(CurrentViewFromCtx(ctx).Filters.View),
		).Show(),
	}
}

func BuildItemsLayout(ctx context.Context, pagination models.Pagination, items models.Items) *HomeLayout {
	cards := make([]templ.Component, 0, len(items))
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard := newItemCard(ctx, item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == CurrentViewFromCtx(ctx).Filters.Count {
			itemCard.addPagination(ctx, pagination)
		}
		// Append the card to the list of cards.
		cards = append(cards, itemCard.Show())
	}
	// Return the home items layout.
	return &HomeLayout{
		Title:   "Items",
		Content: cards,
		Footer: BuildListFooter(ctx,
			models.FeedsRoute,
			items.GetCategoryCounts(),
			addSubscriptionAction(),
			importAction(),
			markAllItemsAction(items.GetFeedIDs(), CurrentViewFromCtx(ctx).Filters.View),
		).Show(),
	}
}
