// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"context"

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
		components.WithTitle(summary.GetTitle(), components.H2),
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
			categories = append(categories,
				components.NewBadge(
					components.WithBadgeDescription(c),
					components.WithSize[components.Badge](components.SM),
					components.WithColor[components.Badge](components.ColorAccent, true)),
			)
		}

		card.Badges = categories
	}

	return card
}

func addImageToCard(card components.Card, img *gofeed.Image) components.Card {
	return components.WithImage(
		components.NewImage(
			components.WithURL(img.URL),
			components.WithAltText(img.Title),
		))(card)
}

func addContentToCard(card components.Card, content string) components.Card {
	return components.WithBody(templ.Raw(content))(card)
}

func NewFeedCard(ctx context.Context, feed Feed) components.Card {
	feedCard := newCard(feed, templ.Attributes{
		"hx-target":  "#" + ContentTarget,
		"hx-get":     "/home/items/show",
		"hx-include": "[id='" + feed.GetID() + "']",
		"hx-headers": `{ ` + HeaderBacklink + `": "` + NavigationFromCtx(ctx).Backlink + `" }`,
	})

	feedCard = addContentToCard(feedCard, feed.GetContent())

	return feedCard
}

func NewItemCard(ctx context.Context, item Item) components.Card {
	return newCard(item, templ.Attributes{
		"hx-target":  "#" + ContentTarget,
		"hx-get":     "/home/article/" + item.GetFeedID() + "/" + item.GetID(),
		"hx-include": "[id='" + item.GetFeedID() + "']",
		"hx-headers": `{ ` + HeaderBacklink + `: "` + NavigationFromCtx(ctx).Backlink + `" }`,
	})
}
