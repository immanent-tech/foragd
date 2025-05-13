// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"net/http"
	"net/url"

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
	viewRoute   *models.Route
	menuActions cardActions
	models.Source
	*card.Props
}

// viewAction returns the action for viewing the card's content. For a Feed card this would be the Feed's item as cards.
// For a Item card, this would be the item content.
func (c *Card) generateViewRoute(ctx context.Context) *models.Route {
	switch models.IdentifyID(c.id) {
	case models.FeedPFX:
		filters := models.FiltersFromCtx(ctx)
		route := models.NewRoute(models.ItemsRoute, &filters, models.WithAttributes(viewAttributes))
		route.SetFeedIDs(c.GetFeedID())
		return route
	case models.ItemPFX:
		return models.NewRoute("/home/"+c.GetFeedID()+"/"+c.id, nil, models.WithAttributes(viewAttributes))
	}
	return nil
}

// AddPagination adds htmx attributes for triggering pagination to a card.
func (c *Card) addPagination(reqURL *url.URL, pagination models.Pagination) {
	action := templates.BuildAction(reqURL.Path,
		templates.WithQueryParams(reqURL.Query()),
		templates.WithAttributes(templ.Attributes{
			"hx-trigger":   "intersect once",
			"hx-swap":      "afterend",
			"hx-push-url":  "false",
			"hx-indicator": "#content-loading",
		}),
	)
	action.AddParameter(models.ParamPagination, pagination)
	c.AddAttributes(action.Attributes())
}

func BuildFeedsLayout(req *http.Request, pagination models.Pagination, subscriptions models.Subscriptions) templates.Layout {
	cards := make([]templ.Component, 0, len(subscriptions))
	// Build feed cards.
	for idx, subscription := range subscriptions {
		card := newFeedCard(req.Context(), subscription)
		if idx == len(subscriptions)-1 && len(subscriptions) == models.FiltersFromCtx(req.Context()).Count {
			card.addPagination(req.URL, pagination)
		}

		cards = append(cards, card.Show())
	}
	back := models.NewRoute("/home", nil)
	return &HomeLayout{
		title:   "Feeds",
		content: cards,
		footer: BuildListFooter(req.Context(),
			BackButton(back),
			subscriptions.GetCategoryCounts(),
			addSubscriptionAction(),
			importAction(),
			markAllFeedsAction(models.FiltersFromCtx(req.Context()).View),
		).Show(),
	}
}

func BuildItemsLayout(req *http.Request, pagination models.Pagination, back *models.Route, items models.Items) templates.Layout {
	cards := make([]templ.Component, 0, len(items))
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard := newItemCard(req.Context(), item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == models.FiltersFromCtx(req.Context()).Count {
			itemCard.addPagination(req.URL, pagination)
		}
		// Append the card to the list of cards.
		cards = append(cards, itemCard.Show())
	}
	// Return the home items layout.
	return &HomeLayout{
		title:   "Items",
		content: cards,
		footer: BuildListFooter(req.Context(),
			BackButton(back),
			items.GetCategoryCounts(),
			addSubscriptionAction(),
			importAction(),
			markAllItemsAction(models.FiltersFromCtx(req.Context()).Feeds, models.FiltersFromCtx(req.Context()).View),
		).Show(),
	}
}
