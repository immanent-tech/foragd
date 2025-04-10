// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"context"
	"net/url"
	"slices"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/classes/opacity"
	"github.com/joshuar/go-templ-daisyui/display/card"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/models"
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
	menuActions cardActions
	models.Source
	*card.Props
}

// setMarkAction creates the appropriate mark action for the card for inclusion in the actions menu.
func (c *Card) setMarkAction() {
	cardType := id.IdentifyID(c.id)
	sourceID := c.id
	switch {
	case cardType == id.Feed && c.IsUnread():
		c.menuActions = append(c.menuActions, partials.MarkButton(buildMarkFeedAction(sourceID, models.MarkRead)))
	case cardType == id.Feed && !c.IsUnread():
		c.menuActions = append(c.menuActions, partials.MarkButton(buildMarkFeedAction(sourceID, models.MarkUnread)))
	case cardType == id.Item && c.IsUnread():
		c.menuActions = append(c.menuActions, partials.MarkButton(buildMarkItemAction(c.GetFeedID(), sourceID, models.MarkRead)))
	case cardType == id.Item && !c.IsUnread():
		c.menuActions = append(c.menuActions, partials.MarkButton(buildMarkItemAction(c.GetFeedID(), sourceID, models.MarkUnread)))
	}
}

func (c *FeedCard) setViewAction(currentFilters models.Filters) {
	filters := models.NewFilters(
		models.WithCountFilter(currentFilters.Count),
		models.WithViewFilter(currentFilters.View),
		models.WithSortFilters(currentFilters.Sort()),
		models.WithFeedFilters(c.GetFeedID()),
	)

	c.menuActions = append(c.menuActions, partials.ViewButton(buildShowItemCardsAction(*filters)))
}

func (c *ItemCard) setViewAction() {
	c.menuActions = append(c.menuActions, partials.ViewButton(buildShowArticleAction(c.GetFeedID(), c.id)))
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
	feedCard.setMarkAction()
	feedCard.setViewAction(models.FiltersFromCtx(ctx))
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

	itemCard.setMarkAction()
	itemCard.setViewAction()

	itemCard.build()

	switch {
	case models.FiltersFromCtx(ctx).View == models.ViewRead:
		fallthrough
	case item.GetUserState() == models.StateRead:
		card.WithExtraClasses(opacity.Apply(75))(itemCard.Props)
	}

	return itemCard
}

func BuildFeeds(ctx context.Context, subscriptions models.Subscriptions) []models.Template {
	content := make([]models.Template, 0, len(subscriptions))
	// Build feed cards.
	for subscription := range slices.Values(subscriptions) {
		content = append(content, BuildFeedCard(ctx, subscription))
	}
	return content
}

func BuildItems(ctx context.Context, reqURL *url.URL, pagination models.Pagination, items models.Items) []models.Template {
	content := make([]models.Template, 0, len(items))
	// Build feed cards.
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard := BuildItemCard(ctx, item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == models.FiltersFromCtx(ctx).Count {
			itemCard.AddPagination(reqURL, pagination)
		}
		// Append the card to the list of cards.
		content = append(content, itemCard)
	}
	return content
}
