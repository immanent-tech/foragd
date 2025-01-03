// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package content

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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

func NewCard(ctx context.Context, item any, count int) (components.Card, error) {
	var (
		title      components.Option[components.Card]
		buttons    []templ.Component
		attributes templ.Attributes
		content    string
	)

	if NavigationFromCtx(ctx).ActionBasePath == "" {
		return components.Card{}, errors.New("could not generate a card: no actions path")
	}

	// Generate type-specific card features.
	switch details := item.(type) {
	case Item:
		actionPath, err := url.JoinPath(NavigationFromCtx(ctx).ChildActionBasePath, "show", details.GetFeedID(), details.GetID())
		if err != nil {
			return components.Card{}, fmt.Errorf("could not generate a card: %w", err)
		}

		title = components.WithTitle(
			details.GetTitle(),
			components.H2)

		buttons = append(buttons,
			buttonToggleItem(details.GetFeedID(), details.GetID()),
			buttonSaveItem(details.GetFeedID(), details.GetID()),
			buttonShareItem(details.GetFeedID(), details.GetID()),
		)

		attributes = templ.Attributes{
			"hx-target":   "#" + ContentTarget,
			"hx-get":      actionPath,
			"hx-push-url": "true",
		}
	case Feed:
		actionPath := NavigationFromCtx(ctx).ChildActionBasePath + "/show?feeds=" + details.GetID()

		title = components.WithTitle(
			details.GetTitle(),
			components.H2,
			components.NewBadge(
				components.WithColor[components.Badge](components.ColorPrimary, false),
				components.WithBadgeDescription(strconv.Itoa(count)),
			))

		content = details.GetContent()

		buttons = append(buttons,
			buttonToggleItem(details.GetID(), ""),
			buttonShareItem(details.GetID(), ""),
		)

		attributes = templ.Attributes{
			"hx-target":   "#" + ContentTarget,
			"hx-get":      actionPath,
			"hx-push-url": "true",
		}
	}

	// Don't continue if we don't have an object that can be represented as a
	// Summary.
	summary, ok := item.(Summary)
	if !ok {
		return components.Card{}, fmt.Errorf("could not generate a card, unknown item")
	}

	// Create the base card with the defined options.
	card := components.NewCard(
		title,
		components.WithBorder(),
		components.WithCardLayout(components.CardLayoutSide),
		components.WithCardShadow(components.XL),
		components.WithID[components.Card](summary.GetID()),
		components.WithAttributes[components.Card](attributes),
		components.WithTopRightActions(withMenu(buttons...)...),
	)

	// If content has been defined, add it to the card.
	if content != "" {
		card = components.WithBody(templ.Raw(content))(card)
	}

	// If there is an image, show it.
	if img := summary.GetImage(); img != nil {
		card = components.WithImage(
			components.NewImage(img.URL,
				components.WithAltText(img.Title),
				components.WithSize[components.Image](components.Size16),
				components.WithMask[components.Image](components.MaskSquircle),
				components.WithObjectFit[components.Image](components.ObjectScaleDown),
			))(card)
	}

	// If there are categories, show them.
	if len(summary.GetCategories()) > 0 {
		var categories []components.Badge
		for _, c := range summary.GetCategories() {
			categories = append(categories,
				components.NewBadge(
					components.WithBadgeDescription(c),
					components.WithResponsiveSize[components.Badge](components.SM),
					components.WithColor[components.Badge](components.ColorAccent, true)),
			)
		}

		card.Badges = categories
	}

	return card, nil
}

func withMenu(items ...templ.Component) []templ.Component {
	var menus []templ.Component

	menus = append(menus,
		// Menu for large screens: horizontal layout, all buttons shown.
		components.NewMenu(
			components.WithResponsiveSize[components.Menu](components.SM),
			components.WithBaseColor[components.Menu](components.ColorBgBase200),
			components.WithLayout[components.Menu](components.HorizontalLayout),
			components.WithRevealedBreakpoint[components.Menu](components.LG),
			components.WithItems[components.Menu](items...),
		).Show(),
		// Menu for small screens: buttons hidden behind drop-down.
		components.NewDropDownMenu(
			components.WithResponsiveSize[components.DropDownMenu](components.SM),
			components.WithBaseColor[components.DropDownMenu](components.ColorBgBase200),
			components.WithLayout[components.DropDownMenu](components.VerticalLayout),
			components.WithHiddenBreakpoint[components.DropDownMenu](components.LG),
			components.WithItems[components.DropDownMenu](items...),
		).Show(),
	)

	return menus
}
