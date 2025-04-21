// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"net/http"
	"net/url"
	"slices"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/classes/opacity"
	"github.com/joshuar/go-templ-daisyui/display/card"

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

type cardActions []templ.Component

// Card is a display component that shows a DaisyUI Card for the given data.
type Card struct {
	id          string
	viewRoute   *models.Route
	menuActions cardActions
	models.Source
	*card.Props
}

// generateMarkAction creates the appropriate mark action for the card.
func (c *Card) generateMarkAction() *templates.Action {
	cardType := models.IdentifyID(c.id)
	sourceID := c.id
	switch {
	case cardType == models.FeedPFX && c.IsUnread():
		return buildMarkFeedAction(sourceID, models.MarkRead)
	case cardType == models.FeedPFX && !c.IsUnread():
		return buildMarkFeedAction(sourceID, models.MarkUnread)
	case cardType == models.ItemPFX && c.IsUnread():
		return buildMarkItemAction(c.GetFeedID(), sourceID, models.MarkRead)
	case cardType == models.ItemPFX && !c.IsUnread():
		return buildMarkItemAction(c.GetFeedID(), sourceID, models.MarkUnread)
	}
	return nil
}

// viewAction returns the action for viewing the card's content. For a Feed card this would be the Feed's item as cards.
// For a Item card, this would be the item content.
func (c *Card) generateViewRoute(ctx context.Context) *models.Route {
	switch models.IdentifyID(c.id) {
	case models.FeedPFX:
		filters := models.FiltersFromCtx(ctx)
		route := models.NewRoute(models.ItemsRoute, &filters)
		route.SetFeedIDs(c.GetFeedID())
		route.SetAttributes(viewAttributes)
		return route
	case models.ItemPFX:
		return models.NewRoute("/home/"+c.GetFeedID()+"/"+c.id, nil)
	}
	return nil
}

// AddPagination adds htmx attributes for triggering pagination to a card.
func (c *Card) AddPagination(reqURL *url.URL, pagination models.Pagination) {
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

func BuildFeedCard(ctx context.Context, subscription *models.Subscription) *FeedCard {
	feedCard := &FeedCard{
		Card: Card{
			Source: subscription,
			id:     subscription.GetFeedID(),
		},
		UnreadCount: subscription.GetUnreadCount(),
	}
	feedCard.viewRoute = feedCard.generateViewRoute(ctx)
	feedCard.menuActions = append(feedCard.menuActions,
		partials.ShareAction(nil),
		partials.VisitExternalLinkAction(models.ParseDomain(subscription.GetLink()), subscription.GetLink()),
		partials.MarkReadAction(feedCard.generateMarkAction()),
	)
	feedCard.build()

	if feedCard.UnreadCount == 0 {
		card.WithExtraClasses(opacity.Apply(75))(feedCard.Props)
	}

	return feedCard
}

func BuildItemCard(ctx context.Context, item *models.Item) *ItemCard {
	itemCard := &ItemCard{
		Card: Card{
			Source: item,
			id:     item.GetID(),
		},
	}
	itemCard.viewRoute = itemCard.generateViewRoute(ctx)
	itemCard.menuActions = append(itemCard.menuActions,
		showLastUpdated("", item.GetUpdatedDate()),
		partials.MarkButton(itemCard.generateMarkAction()),
	)
	itemCard.build()

	switch {
	case models.FiltersFromCtx(ctx).View == models.ViewRead:
		fallthrough
	case item.GetUserState() == models.StateRead:
		card.WithExtraClasses(opacity.Apply(75))(itemCard.Props)
	}

	return itemCard
}

func BuildFeeds(req *http.Request, subscriptions models.Subscriptions) templ.Component {
	content := make(Cards, 0, len(subscriptions))
	// Build feed cards.
	for subscription := range slices.Values(subscriptions) {
		content = append(content, BuildFeedCard(req.Context(), subscription).Show())
	}
	return content.Render(req)
}

func BuildItems(req *http.Request, pagination models.Pagination, items models.Items) templ.Component {
	content := make(Cards, 0, len(items))
	// Build feed cards.
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard := BuildItemCard(req.Context(), item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == models.FiltersFromCtx(req.Context()).Count {
			itemCard.AddPagination(req.URL, pagination)
		}
		// Append the card to the list of cards.
		content = append(content, itemCard.Show())
	}
	return content.Render(req)
}
