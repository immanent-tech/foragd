// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package panes

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
	"github.com/joshuar/go-templ-daisyui/display/text"
	"github.com/joshuar/go-templ-daisyui/layout/mask"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"
	"github.com/joshuar/go-templ-daisyui/modifiers/size"
	"github.com/joshuar/go-templ-daisyui/navigation/menu"
	"github.com/mmcdole/gofeed"

	"github.com/joshuar/go-feed-me/internal/models"
)

const (
	ContentTarget = "content"
)

func RelativeTime(timestamp time.Time) string {
	return timestamp.Format("Mon Jan 2 15:04")
}

type Feed interface {
	Summary
	Content
	GetUnreadCount() int
}

type Item interface {
	Summary
	Content
	GetFeedID() string
	GetState() models.State
}

type Summary interface {
	GetTitle() string
	GetID() string
	GetImage() *gofeed.Image
	GetCategories() []string
	GetTimestamp() time.Time
	GetLink() string
}

type Content interface {
	GetContent() string
}

type cardCustomisation struct {
	title     components.Option[*components.CardProps]
	actions   []templ.Component
	content   templ.Component
	updatedAt string
}

func NewCard(ctx context.Context, item any) (*components.CardProps, error) {
	var customisation *cardCustomisation

	// Don't continue if we don't have an object that can be represented as a
	// Summary.
	summary, ok := item.(Summary)
	if !ok {
		return nil, fmt.Errorf("could not generate a card, unknown item")
	}

	// Generate type-specific card customisation.
	switch details := item.(type) {
	case Item:
		customisation = itemCustomisation(ctx, details)
	case Feed:
		customisation = feedCustomisation(ctx, details)
	}

	var categories []templ.Component
	// If there are categories, show them.
	// if len(summary.GetCategories()) > 0 {
	// 	for _, c := range summary.GetCategories() {
	// 		categories = append(categories,
	// 			components.Badge(
	// 				components.WithResponsiveSize[*components.BadgeProps](components.SM),
	// 				components.WithColor[*components.BadgeProps](components.ColorAccent, true)),
	// 		)
	// 	}
	// }

	// Create the base CardProps with the defined options.
	cardProps := components.BuildCard(
		// customisation.title,
		components.WithBorder(),
		components.WithCardLayout(components.CardLayoutSide),
		components.WithCardShadow(components.XL),
		components.WithCompactCardBody(),
		components.WithID[*components.CardProps](summary.GetID()),
		components.WithBody(customisation.content,
			components.WithTopRightActions(withMenu(customisation.actions...)...),
			components.WithBottomLeftActions(text.Build(
				customisation.updatedAt,
				text.AsItalicText(),
				text.WithTextSize(text.SM)).Show()),
			components.WithBottomRightActions(categories...),
			components.WithAttributes[*components.CardBodyProps](templ.Attributes{
				"hx-target":   "#" + ContentTarget,
				"hx-push-url": "true",
			}),
		),
	)

	// If there is an image, show it.
	if img := summary.GetImage(); img != nil {
		cardProps = components.WithImage(img.URL,
			components.ImageTop,
			components.WithLazyLoading(),
			components.WithAltText(img.Title),
			components.WithMask(mask.MaskSquircle),
		)(cardProps)
	}

	return cardProps, nil
}

func withMenu(items ...templ.Component) []templ.Component {
	var menus []templ.Component

	menus = append(menus,
		// Menu for large screens: horizontal layout, all buttons shown.
		menu.Build(
			menu.WithSize(size.SM),
			menu.WithBaseColor(color.Base200),
			menu.WithLayout(menu.Horizontal),
			menu.WithRevealedBreakpoint(size.LG),
		).Show(items...),
		// Menu for small screens: buttons hidden behind drop-down.
		menu.Build(
			menu.WithSize(size.SM),
			menu.WithBaseColor(color.Base200),
			menu.WithLayout(menu.Vertical),
			menu.WithHiddenBreakpoint(size.LG),
		).Show(items...),
	)

	return menus
}

func itemCustomisation(ctx context.Context, item Item) *cardCustomisation {
	var actions []templ.Component

	switch item.GetState() {
	case models.StateRead:
		path, err := url.JoinPath(models.ItemSetBasePathFromCtx(ctx), item.GetFeedID(), item.GetID(), string(models.StateUnread))
		if err == nil {
			actions = append(actions, actionButton(path, "#"+item.GetID(), "fa-file", "Mark Item Unread"))
		}
	case models.StateUnread:
		fallthrough
	default:
		path, err := url.JoinPath(models.ItemSetBasePathFromCtx(ctx), item.GetFeedID(), item.GetID(), string(models.StateUnread))
		if err != nil {
			actions = append(actions, actionButton(path, "#"+item.GetID(), "fa-check", "Mark Item Read"))
		}
	}

	return &cardCustomisation{
		title: components.WithTitle(
			item.GetTitle(),
			components.H4),
		actions: actions,

		// 	buttonToggleItem(item.GetFeedID(), item.GetID()),
		// 	buttonSaveItem(item.GetFeedID(), item.GetID()),
		// 	buttonShareItem(item.GetFeedID(), item.GetID()),
		content: text.Build(
			item.GetTitle(),
			text.WithTextSize(text.LG),
			text.WithTextWeight(text.Semibold)).Show(),
		updatedAt: "Published: " + RelativeTime(item.GetTimestamp()),
	}
}

func feedCustomisation(ctx context.Context, feed Feed) *cardCustomisation {
	markReadURL := models.SetQueryParams(
		models.PageNavigationFromCtx(ctx).GenerateActionURL(models.StateRead),
		map[string]string{
			"feeds": feed.GetID(),
		})

	return &cardCustomisation{
		title: components.WithTitle(
			feed.GetTitle(),
			components.H2),
		content: FeedCard(feed),
		actions: []templ.Component{
			actionButton(markReadURL.String(), "#"+feed.GetID(), "fa-check", "Mark Feed Read"),
		},
		updatedAt: "Last Update: " + RelativeTime(feed.GetTimestamp()),
	}
}
