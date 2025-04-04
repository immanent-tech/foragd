// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/classes/opacity"
	"github.com/joshuar/go-templ-daisyui/display/card"
	"github.com/joshuar/go-templ-daisyui/display/image"
	"github.com/joshuar/go-templ-daisyui/layout/mask"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
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
func (c *Card) buildMarkButton(label string, mark models.Mark, path string) templ.Component {
	// Build action options.
	action := templates.BuildAction(path,
		templates.WithAttributes(templ.Attributes{
			"_":           "on click halt the event's bubbling",
			"hx-push-url": "false",
			"hx-target":   c.Target,
		}),
		templates.WithMethod(http.MethodPost),
	)

	// Set query parameters based on card type.
	switch c.Type {
	case Feed:
		action.AddParameter(models.ParamFeeds, c.GetID())
		action.AddParameter("mark", string(mark))
	case Item:
		action.AddParameter(models.ParamItems, c.GetID())
		action.AddParameter("mark", string(mark))
	}

	return button.Build(
		button.WithSize(size.XS),
		button.WithContent(label),
		button.WithExtraAttributes(action.Attributes()),
	).Show()
}

// buildRoute creates a models.Route appropriate for showing content.
func (c *Card) buildRoute(path string, filters models.Filters) *templates.Action {
	// Build action options.
	action := templates.BuildAction(path,
		templates.WithAttributes(templ.Attributes{
			"hx-swap":     "morph:innerHTML",
			"hx-push-url": "false",
			"hx-target":   c.Target,
		}),
		templates.WithMethod(http.MethodPost),
	)
	action.AddParameter(models.ParamView, string(filters.View))
	action.AddParameter(models.ParamCount, strconv.Itoa(filters.Count))

	switch c.Type {
	case Feed:
		action.AddParameter(models.ParamFeeds, c.GetID())
	}

	return action
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

func NewFeedCard(filters models.Filters, subscription *models.Subscription) (*Card, error) {
	feedCard := &Card{
		Type:   Feed,
		ID:     subscription.GetFeedID(),
		Target: ContentID.Target(),
	}

	// // Embed the Feed.
	// feedCard.Data, err = json.Marshal(feed)
	// if err != nil {
	// 	return nil, errors.Join(ErrNewCard, err)
	// }
	// Build a route for showing feed items.
	route := feedCard.buildRoute("/home/items", filters)
	// Add card menu item for marking read/unread.
	var cardMenuItems []templ.Component
	if subscription.GetUnreadCount() > 0 {
		cardMenuItems = append(cardMenuItems,
			feedCard.buildMarkButton("Mark Read", models.MarkRead, models.FeedsRoute))
	} else {
		cardMenuItems = append(cardMenuItems,
			feedCard.buildMarkButton("Mark Unread", models.MarkUnread, models.FeedsRoute))
	}
	// Build card options.
	var cardOptions []card.Option
	cardOptions = append(cardOptions,
		card.WithLayout(card.LayoutSide),
		card.Bordered(),
		card.WithShadow(size.XL),
		card.WithBodyOptions(
			card.WithContent(showFeedCardContent(subscription)),
			card.WithActions(showCardActions(cardMenuItems...)...),
			card.WithBodyExtraAttributes(route.Attributes()),
		),
		card.WithID(subscription.GetFeedID()),
	)
	// Add an image if present.
	if cardImage := subscription.Feed.GetImage(); cardImage != nil {
		cardOptions = append(cardOptions,
			card.WithImage(cardImage.URL(),
				image.WithAltText(cardImage.String()),
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
	if subscription.GetUnreadCount() == 0 {
		cardOptions = append(cardOptions, card.WithExtraClasses(opacity.Apply(75)))
	}

	feedCard.Props = card.Build(cardOptions...)

	return feedCard, nil
}

func NewItemCard(filters models.Filters, item *models.Item) (*Card, error) {
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
	if item.GetUserState() == models.StateUnread {
		cardMenuItems = append(cardMenuItems,
			itemCard.buildMarkButton("Mark Read", models.MarkRead, "/home/items"))
	} else {
		cardMenuItems = append(cardMenuItems,
			itemCard.buildMarkButton("Mark Unread", models.MarkUnread, "/home/items"))
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
			card.WithImage(itemImage.URL(),
				image.WithAltText(itemImage.String()),
				image.WithLazyLoading(),
				image.WithMask(mask.MaskSquircle),
			),
		)
	}
	// Reduce opacity if item is read or view is read items.
	switch {
	case filters.ViewRead():
		fallthrough
	case item.GetUserState() == models.StateRead:
		cardOptions = append(cardOptions, card.WithExtraClasses(opacity.Apply(75)))
	}

	itemCard.Props = card.Build(cardOptions...)

	return itemCard, nil
}
