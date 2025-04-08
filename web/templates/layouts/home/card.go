// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

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
	actionPath  string
	filters     models.Filters
	target      string
	menuActions cardActions
	models.Source
	*card.Props
}

// setCardAttributes will set the additional attributes that control the action that happens when the card is clicked.
func (c *Card) setCardAttributes(feeds ...models.FeedID) templ.Attributes {
	// Build action options.
	action := templates.BuildAction(c.actionPath,
		templates.WithAttributes(templ.Attributes{
			"hx-swap":     "morph:innerHTML",
			"hx-push-url": "true",
			"hx-target":   c.target,
		}),
		templates.WithMethod(http.MethodGet),
	)
	action.AddParameter(models.ParamView, string(c.filters.View))
	action.AddParameter(models.ParamCount, strconv.Itoa(c.filters.Count))
	if len(feeds) > 0 {
		action.AddParameter(models.ParamFeeds, strings.Join(feeds, ","))
	}

	return action.Attributes()
}

// setMarkAction creates the appropriate mark action for the card for inclusion in the actions menu.
func (c *Card) setMarkAction() {
	var paramName string
	switch id.IdentifyID(c.id) {
	case id.Feed:
		paramName = models.ParamFeeds
	case id.Item:
		paramName = models.ParamItems
	}
	if c.IsUnread() {
		c.menuActions = append(c.menuActions,
			partials.MarkReadButton(models.FeedsRoute, c.target, url.Values{paramName: []string{c.id}}),
		)
	} else {
		c.menuActions = append(c.menuActions,
			partials.MarkUnreadButton(models.FeedsRoute, c.target, url.Values{paramName: []string{c.id}}),
		)
	}
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

func BuildFeedCard(filters models.Filters, subscription *models.Subscription) *FeedCard {
	feedCard := &FeedCard{
		Card: Card{
			Source:     subscription,
			actionPath: "/home/items",
			filters:    filters,
			target:     ContentID.Target(),
			id:         subscription.GetFeedID(),
		},
		UnreadCount: subscription.GetUnreadCount(),
	}

	feedCard.setMarkAction()
	feedCard.menuActions = append(feedCard.menuActions, partials.ViewButton(feedCard.setCardAttributes(feedCard.GetFeedID())))

	feedCard.build()

	if feedCard.UnreadCount == 0 {
		card.WithExtraClasses(opacity.Apply(75))(feedCard.Props)
	}

	return feedCard
}

func BuildItemCard(filters models.Filters, item *models.Item) *ItemCard {
	itemCard := &ItemCard{
		Card: Card{
			Source:     item,
			actionPath: "/home/" + item.GetFeedID() + "/" + item.GetID(),
			filters:    filters,
			target:     ContentID.Target(),
			id:         item.GetID(),
		},
	}

	itemCard.setMarkAction()

	itemCard.build()

	switch {
	case filters.ViewRead():
		fallthrough
	case item.GetUserState() == models.StateRead:
		card.WithExtraClasses(opacity.Apply(75))(itemCard.Props)
	}

	return itemCard
}
