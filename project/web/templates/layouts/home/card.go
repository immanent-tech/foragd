// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/classes/opacity"
	"github.com/joshuar/go-templ-daisyui/display/card"
	"github.com/joshuar/go-templ-daisyui/display/image"
	"github.com/joshuar/go-templ-daisyui/layout/mask"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/models"
)

var ErrNewCard = errors.New("could not create new card")

const (
	Feed CardType = iota
	Item
)

type CardType int

// Card is a display component that shows a DaisyUI Card for the given data.
type Card struct {
	ID     string
	Target string
	Type   CardType
	Data   json.RawMessage
	*card.Props
}

// buildMarkButton creates a button to mark the card as read/unread.
func (c *Card) buildMarkButton(label string, mark api.Mark, path string) templ.Component {
	var paramOption api.RouteOption

	switch c.Type {
	case Feed:
		paramOption = api.WithParams(
			api.WithFeedsParam(c.ID),
			api.WithMarkParam(mark),
		)
	case Item:
		paramOption = api.WithParams(
			api.WithItemsParam(c.ID),
			api.WithMarkParam(mark),
		)
	}

	route := api.BuildRoute(path,
		api.WithAttributes(templ.Attributes{
			"_":           "on click halt the event's bubbling",
			"hx-push-url": "false",
			"hx-target":   c.Target,
		}),
		paramOption,
		api.WithMethod(http.MethodPost),
	)

	return button.Build(
		button.WithSize(size.XS),
		button.WithContent(label),
		button.WithExtraAttributes(route.Attributes()),
	).Show()
}

// buildRoute creates a models.Route appropriate for showing content.
func (c *Card) buildRoute(path string, filters api.Filters) *api.Route {
	var routeOptions []api.RouteOption

	routeOptions = append(routeOptions,
		api.WithAttributes(templ.Attributes{
			"hx-push-url": "true",
			"hx-target":   c.Target,
			"hx-swap":     "morph:innerHTML",
		}),
		api.WithParams(
			api.WithViewParam(filters.View),
			api.WithCountParam(filters.Count),
		),
	)

	switch c.Type {
	case Feed:
		routeOptions = append(routeOptions,
			api.WithParams(
				api.WithFeedsParam(c.ID),
			),
		)
	}

	return api.BuildRoute(path, routeOptions...)
}

// AddPagination adds htmx attributes for triggering pagination to a card.
func (c *Card) AddPagination(path *url.URL, pagination api.Pagination) {
	paginateRoute := api.BuildRoute(path,
		api.WithParams(api.WithPaginationParam(pagination)),
	).String()
	c.AddAttributes(templ.Attributes{
		"hx-get":       paginateRoute,
		"hx-trigger":   "intersect once",
		"hx-swap":      "afterend",
		"hx-push-url":  "false",
		"hx-indicator": "#content-loading",
	})
}

func NewFeedCard(filters api.Filters, feed *models.APIFeed) (*Card, error) {
	var err error

	feedCard := &Card{
		Type:   Feed,
		ID:     feed.GetID(),
		Target: ContentID.Target(),
	}

	// Embed the Feed.
	feedCard.Data, err = json.Marshal(feed)
	if err != nil {
		return nil, errors.Join(ErrNewCard, err)
	}
	// Build a route for showing feed items.
	route := feedCard.buildRoute("/home/items", filters)
	// Add card menu item for marking read/unread.
	var cardMenuItems []templ.Component
	if feed.GetUserUnreadCount() > 0 {
		cardMenuItems = append(cardMenuItems,
			feedCard.buildMarkButton("Mark Read", api.MarkRead, api.FeedsRoute))
	} else {
		cardMenuItems = append(cardMenuItems,
			feedCard.buildMarkButton("Mark Unread", api.MarkUnread, api.FeedsRoute))
	}
	// Build card options.
	var cardOptions []card.Option
	cardOptions = append(cardOptions,
		card.WithLayout(card.LayoutSide),
		card.Bordered(),
		card.WithShadow(size.XL),
		card.WithBodyOptions(
			card.WithContent(showFeedCardContent(feed)),
			card.WithActions(showCardActions(cardMenuItems...)...),
			card.WithBodyExtraAttributes(route.Attributes()),
		),
		card.WithID(feed.GetID()),
	)
	// Add an image if present.
	if cardImage := feed.GetImage(); cardImage != nil {
		cardOptions = append(cardOptions,
			card.WithImage(cardImage.URL,
				image.WithAltText(cardImage.Title),
				image.WithLazyLoading(),
				image.WithMask(mask.MaskSquircle),
			),
		)
	} else {
		cardOptions = append(cardOptions,
			card.WithImage("/static/images/square-rss-solid.svg",
				image.WithLazyLoading(),
			),
		)
	}
	// Reduce opacity if feed is read.
	if feed.GetUserUnreadCount() == 0 {
		cardOptions = append(cardOptions, card.WithExtraClasses(opacity.Apply(75)))
	}

	feedCard.Props = card.Build(cardOptions...)

	return feedCard, nil
}

func NewItemCard(filters api.Filters, item *models.APIItem) (*Card, error) {
	var err error

	itemCard := &Card{
		Type:   Item,
		ID:     item.GetID(),
		Target: ContentID.Target(),
	}

	// Embed the Item.
	itemCard.Data, err = json.Marshal(item)
	if err != nil {
		return nil, errors.Join(ErrNewCard, err)
	}
	// Create a route for showing item.
	route := itemCard.buildRoute("/home/"+item.GetFeedID()+"/"+item.GetID(), filters)
	// Add card menu item for marking read/unread.
	var cardMenuItems []templ.Component
	if item.GetUserState() == models.Unread {
		cardMenuItems = append(cardMenuItems,
			itemCard.buildMarkButton("Mark Read", api.MarkRead, "/home/items"))
	} else {
		cardMenuItems = append(cardMenuItems,
			itemCard.buildMarkButton("Mark Unread", api.MarkUnread, "/home/items"))
	}
	// Build card options.
	var cardOptions []card.Option
	cardOptions = append(cardOptions,
		card.Bordered(),
		card.WithShadow(size.XL),
		card.WithBodyOptions(
			card.WithContent(showItemCardContent(item)),
			card.WithActions(showCardActions(cardMenuItems...)...),
			card.WithBodyExtraAttributes(route.Attributes()),
		),
		card.WithID(item.GetID()),
	)
	// Add an image if present.
	if itemImage := item.GetImage(); itemImage != nil {
		cardOptions = append(cardOptions,
			card.WithImage(itemImage.URL,
				image.WithAltText(itemImage.Title),
				image.WithLazyLoading(),
				image.WithMask(mask.MaskSquircle),
			),
		)
	}
	// Reduce opacity if item is read or view is read items.
	switch {
	case filters.ViewRead():
		fallthrough
	case item.GetUserState() == models.Read:
		cardOptions = append(cardOptions, card.WithExtraClasses(opacity.Apply(75)))
	}

	itemCard.Props = card.Build(cardOptions...)

	return itemCard, nil
}
