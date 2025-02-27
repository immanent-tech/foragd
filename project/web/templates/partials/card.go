// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/attributes"
	"github.com/joshuar/go-templ-daisyui/classes/opacity"
	"github.com/joshuar/go-templ-daisyui/display/card"
	"github.com/joshuar/go-templ-daisyui/display/image"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"

	"github.com/joshuar/go-feed-me/internal/models"
)

var ErrNewCard = errors.New("could not create new card")

const (
	ContentTarget = "#content"
)

const (
	Feed CardType = iota
	Item
)

type CardType int

type Card struct {
	ID     string
	Target string
	Type   CardType
	Data   json.RawMessage
	*card.Props
}

// buildMarkButton creates a button to mark the card as read/unread.
func (c *Card) buildMarkButton(label string, mark models.Mark, path string) templ.Component {
	var paramOption models.RouteOption

	switch c.Type {
	case Feed:
		paramOption = models.WithParams(
			models.WithFeedsParam(c.ID),
			models.WithMarkParam(mark),
		)
	case Item:
		paramOption = models.WithParams(
			models.WithItemsParam(c.ID),
			models.WithMarkParam(mark),
		)
	}

	route := models.BuildRoute(path,
		models.WithAttributes(templ.Attributes{
			"_":           "on click halt the event's bubbling",
			"hx-push-url": "false",
			"hx-target":   c.Target,
		}),
		paramOption,
		models.WithMethod(http.MethodPost),
	)

	return button.Build(
		button.WithSize(size.XS),
		button.WithContent(label),
		button.WithExtraAttributes(route.Attributes()),
	).Show()
}

// buildProps builds the card.Props for the given content.
func (c *Card) buildProps(ctx context.Context, path string, filters models.APIFilters, options ...card.Option) {
	var routeOptions []models.RouteOption

	routeOptions = append(routeOptions,
		models.WithAttributes(templ.Attributes{
			"hx-push-url": "true",
			"hx-target":   c.Target,
		}),
		models.WithParams(
			models.WithViewParam(filters.View),
			models.WithCountParam(filters.Count),
		),
	)

	switch c.Type {
	case Feed:
		routeOptions = append(routeOptions,
			models.WithParams(
				models.WithFeedsParam(c.ID),
			),
		)
	}

	route := models.BuildRoute(path, routeOptions...)

	options = append(options,
		card.Bordered(),
		card.WithShadow(size.XL),
		card.WithExtraAttributes(route.Attributes()),
	)

	c.Props = card.Build(options...)
}

func NewFeedCard(ctx context.Context, filters models.APIFilters, feed *models.APIFeed) (*Card, error) {
	var err error

	feedCard := &Card{
		Type:   Feed,
		ID:     feed.GetID(),
		Target: ContentTarget,
	}

	// Embed the Feed.
	feedCard.Data, err = json.Marshal(feed)
	if err != nil {
		return nil, errors.Join(ErrNewCard, err)
	}

	var cardMenuItems []templ.Component
	// Add card menu item for marking read/unread.
	if feed.GetUserUnreadCount() > 0 {
		cardMenuItems = append(cardMenuItems,
			feedCard.buildMarkButton("Mark Read", models.MarkRead, "/home/feeds"))
	} else {
		cardMenuItems = append(cardMenuItems,
			feedCard.buildMarkButton("Mark Unread", models.MarkUnread, "/home/feeds"))
	}

	var cardOptions []card.Option
	// Build card options.
	cardOptions = append(cardOptions,
		card.WithLayout(card.LayoutSide),
		card.WithBodyOptions(
			card.WithContent(showFeedCardContent(feed)),
			card.WithActions(showCardActions(cardMenuItems...)...),
		),
		card.WithID(attributes.ID(feed.GetID())),
	)
	// Add an image if present.
	if cardImage := feed.GetImage(); cardImage != nil {
		cardOptions = append(cardOptions,
			card.WithImage(cardImage.URL,
				image.WithAltText(cardImage.Title),
				image.WithLazyLoading(),
			),
		)
	}
	// Reduce opacity if feed is read.
	if feed.GetUserUnreadCount() == 0 {
		cardOptions = append(cardOptions, card.WithExtraClasses(opacity.Apply(75)))
	}

	feedCard.buildProps(ctx, "/home/items", filters, cardOptions...)

	return feedCard, nil
}

func NewItemCard(ctx context.Context, filters models.APIFilters, item *models.APIItem) (*Card, error) {
	var err error

	itemCard := &Card{
		Type:   Item,
		ID:     item.GetID(),
		Target: ContentTarget,
	}

	// Embed the Item.
	itemCard.Data, err = json.Marshal(item)
	if err != nil {
		return nil, errors.Join(ErrNewCard, err)
	}

	var cardMenuItems []templ.Component
	// Add card menu item for marking read/unread.
	if item.GetUserState() == models.Unread {
		cardMenuItems = append(cardMenuItems,
			itemCard.buildMarkButton("Mark Read", models.MarkRead, "/home/items"))
	} else {
		cardMenuItems = append(cardMenuItems,
			itemCard.buildMarkButton("Mark Unread", models.MarkUnread, "/home/items"))
	}

	var cardOptions []card.Option
	// Build card options.
	cardOptions = append(cardOptions,
		card.WithBodyOptions(
			card.WithContent(showItemCardContent(item)),
			card.WithActions(showCardActions(cardMenuItems...)...),
		),
		card.WithID(attributes.ID(item.GetID())),
	)
	// Add an image if present.
	if itemImage := item.GetImage(); itemImage != nil {
		cardOptions = append(cardOptions,
			card.WithImage(itemImage.URL,
				image.WithAltText(itemImage.Title),
				image.WithLazyLoading(),
			),
		)
	}
	// Reduce opacity if item is read or view is read items.
	switch {
	case filters.View == models.ViewRead:
		fallthrough
	case item.GetUserState() == models.Read:
		cardOptions = append(cardOptions, card.WithExtraClasses(opacity.Apply(75)))
	}

	itemCard.buildProps(ctx, "/home/"+item.GetFeedID()+"/"+item.GetID(), filters, cardOptions...)

	return itemCard, nil
}
