// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"fmt"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/dustin/go-humanize"
	components "github.com/joshuar/go-templ-daisyui"
	"github.com/mmcdole/gofeed"
)

const (
	ContentTarget = "content-main"
)

func RelativeTime(timestamp time.Time) string {
	return humanize.Time(timestamp)
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
	title   components.Option[*components.CardProps]
	buttons []templ.Component
	content string
}

func NewCard(item any) (*components.CardProps, error) {
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
		customisation = itemCustomisation(details)
	case Feed:
		customisation = feedCustomisation(details)
	}

	// Create the base CardProps with the defined options.
	cardProps := components.BuildCard(
		customisation.title,
		components.WithBorder(),
		components.WithCardLayout(components.CardLayoutSide),
		components.WithCardShadow(components.XL),
		components.WithID[*components.CardProps](summary.GetID()),
		components.WithTopRightActions(withMenu(customisation.buttons...)...),
		components.WithBody(templ.Raw(customisation.content), templ.Attributes{
			"hx-target":   "#" + ContentTarget,
			"hx-push-url": "true",
		}),
	)

	// If there is an image, show it.
	if img := summary.GetImage(); img != nil {
		cardProps = components.WithImage(img.URL,
			components.WithAltText(img.Title),
			components.WithSize[*components.ImageProps](components.Size16),
			components.WithMask[*components.ImageProps](components.MaskSquircle),
			components.WithObjectFit[*components.ImageProps](components.ObjectScaleDown),
		)(cardProps)
	}

	// If there are categories, show them.
	if len(summary.GetCategories()) > 0 {
		var categories []templ.Component
		for _, c := range summary.GetCategories() {
			categories = append(categories,
				components.Badge(
					components.WithBadgeDescription(c),
					components.WithResponsiveSize[*components.BadgeProps](components.SM),
					components.WithColor[*components.BadgeProps](components.ColorAccent, true)),
			)
		}

		cardProps.Badges = categories
	}

	return cardProps, nil
}

func withMenu(items ...templ.Component) []templ.Component {
	var menus []templ.Component

	menus = append(menus,
		// Menu for large screens: horizontal layout, all buttons shown.
		components.NewMenu(
			components.WithResponsiveSize[*components.Menu](components.SM),
			components.WithBaseColor[*components.Menu](components.ColorBgBase200),
			components.WithLayout[*components.Menu](components.HorizontalLayout),
			components.WithRevealedBreakpoint[*components.Menu](components.LG),
			components.WithItems[*components.Menu](items...),
		).Show(),
		// Menu for small screens: buttons hidden behind drop-down.
		components.NewDropDownMenu(
			components.WithResponsiveSize[*components.DropDownMenu](components.SM),
			components.WithBaseColor[*components.DropDownMenu](components.ColorBgBase200),
			components.WithLayout[*components.DropDownMenu](components.VerticalLayout),
			components.WithHiddenBreakpoint[*components.DropDownMenu](components.LG),
			components.WithItems[*components.DropDownMenu](items...),
		).Show(),
	)

	return menus
}

func itemCustomisation(item Item) *cardCustomisation {
	return &cardCustomisation{
		title: components.WithTitle(
			item.GetTitle(),
			components.H2),
		buttons: []templ.Component{
			buttonToggleItem(item.GetFeedID(), item.GetID()),
			buttonSaveItem(item.GetFeedID(), item.GetID()),
			buttonShareItem(item.GetFeedID(), item.GetID()),
		},
	}
}

func feedCustomisation(feed Feed) *cardCustomisation {
	return &cardCustomisation{
		title: components.WithTitle(
			feed.GetTitle(),
			components.H2,
			components.Badge(
				components.WithColor[*components.BadgeProps](components.ColorPrimary, false),
				components.WithBadgeDescription(strconv.Itoa(feed.GetUnreadCount())),
			)),
		content: feed.GetContent(),
		buttons: []templ.Component{
			buttonToggleItem(feed.GetID(), ""),
			buttonShareItem(feed.GetID(), ""),
		},
	}
}
