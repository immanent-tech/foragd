// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
	"github.com/mmcdole/gofeed"
)

type Feed interface {
	Summary
	Content
}

type Item interface {
	Summary
	Content
	GetFeedID() string
}

type Summary interface {
	GetTitle() string
	GetID() string
	GetImage() *gofeed.Image
	GetCategories() []string
}

type Content interface {
	GetContent() string
}

func newCard(summary Summary, attributes templ.Attributes) components.Card {
	card := components.NewCard(
		components.WithBorder(),
		components.WithCardLayout(components.CardLayoutSide),
		components.WithTitle(summary.GetTitle()),
		components.WithCardShadow(components.XL),
		components.WithID[components.Card](summary.GetID()),
		components.WithAttributes[components.Card](attributes),
	)

	if summary.GetImage() != nil {
		card = addImageToCard(card, summary.GetImage())
	}

	if len(summary.GetCategories()) > 0 {
		var categories []components.Badge
		for _, c := range summary.GetCategories() {
			categories = append(categories, components.NewBadge(c))
		}

		card.Badges = categories
	}

	return card
}

func addImageToCard(card components.Card, img *gofeed.Image) components.Card {
	image := components.NewImage(
		components.WithURL(img.URL),
	)

	if img.Title != "" {
		image.Alt = img.Title
	}

	return components.WithImage(image)(card)
}

func addContentToCard(card components.Card, content string) components.Card {
	return components.WithBody(templ.Raw(content))(card)
}

func NewFeedCard(feed Feed) components.Card {
	feedCard := newCard(feed, templ.Attributes{
		"hx-target":  "#content",
		"hx-get":     "/home/items/show",
		"hx-include": "[id='" + feed.GetID() + "'], [name='backlink']",
	})

	feedCard = addContentToCard(feedCard, feed.GetContent())

	return feedCard
}

func NewItemCard(item Item) components.Card {
	return newCard(item, templ.Attributes{
		"hx-target":  "#content",
		"hx-get":     "/home/" + item.GetFeedID() + "-" + item.GetID(),
		"hx-include": "[id='" + item.GetFeedID() + "'], [name='backlink']",
	})
}
