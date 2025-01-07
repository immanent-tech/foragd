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

type cardCustomisation struct {
	title      components.Option[components.Card]
	buttons    []templ.Component
	attributes templ.Attributes
	content    string
}

func NewCard(ctx context.Context, item any, count int) (components.Card, error) {
	var (
		customisation *cardCustomisation
		err           error
	)

	// Don't continue if we cannot generate card menu actions.
	if NavigationFromCtx(ctx).ActionBasePath == "" {
		return components.Card{}, errors.New("could not generate a card: no actions path")
	}

	// Don't continue if we don't have an object that can be represented as a
	// Summary.
	summary, ok := item.(Summary)
	if !ok {
		return components.Card{}, fmt.Errorf("could not generate a card, unknown item")
	}

	// Generate type-specific card customisation.
	switch details := item.(type) {
	case Item:
		customisation, err = itemCustomisation(ctx, details)
	case Feed:
		customisation, err = feedCustomisation(ctx, details, count)
	}

	if err != nil {
		return components.Card{}, fmt.Errorf("could not generate a card: %w", err)
	}

	// Create the base card with the defined options.
	card := components.NewCard(
		customisation.title,
		components.WithBorder(),
		components.WithCardLayout(components.CardLayoutSide),
		components.WithCardShadow(components.XL),
		components.WithID[components.Card](summary.GetID()),
		components.WithAttributes[components.Card](customisation.attributes),
		components.WithTopRightActions(withMenu(customisation.buttons...)...),
	)

	// If content has been defined, add it to the card.
	if customisation.content != "" {
		card = components.WithBody(templ.Raw(customisation.content))(card)
	}

	// If there is an image, show it.
	if img := summary.GetImage(); img != nil {
		card = components.WithImage(img.URL,
			components.WithAltText(img.Title),
			components.WithSize[components.ImageProps](components.Size16),
			components.WithMask[components.ImageProps](components.MaskSquircle),
			components.WithObjectFit[components.ImageProps](components.ObjectScaleDown),
		)(card)
	}

	// If there are categories, show them.
	if len(summary.GetCategories()) > 0 {
		var categories []templ.Component
		for _, c := range summary.GetCategories() {
			categories = append(categories,
				components.Badge(
					components.WithBadgeDescription(c),
					components.WithResponsiveSize[components.BadgeProps](components.SM),
					components.WithColor[components.BadgeProps](components.ColorAccent, true)),
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

func itemCustomisation(ctx context.Context, item Item) (*cardCustomisation, error) {
	actionPath, err := url.JoinPath(NavigationFromCtx(ctx).ChildActionBasePath, item.GetFeedID(), item.GetID())
	if err != nil {
		return nil, fmt.Errorf("could not generate a card: %w", err)
	}

	return &cardCustomisation{
			attributes: templ.Attributes{
				"hx-target":   "#" + ContentTarget,
				"hx-get":      actionPath,
				"hx-push-url": "true",
			},
			title: components.WithTitle(
				item.GetTitle(),
				components.H2),
			buttons: []templ.Component{
				buttonToggleItem(item.GetFeedID(), item.GetID()),
				buttonSaveItem(item.GetFeedID(), item.GetID()),
				buttonShareItem(item.GetFeedID(), item.GetID()),
			},
		},
		nil
}

func feedCustomisation(ctx context.Context, feed Feed, count int) (*cardCustomisation, error) {
	actionPath := NavigationFromCtx(ctx).ChildActionBasePath + "?feeds=" + feed.GetID()

	return &cardCustomisation{
			attributes: templ.Attributes{
				"hx-target":   "#" + ContentTarget,
				"hx-get":      actionPath,
				"hx-push-url": "true",
			},
			title: components.WithTitle(
				feed.GetTitle(),
				components.H2,
				components.Badge(
					components.WithColor[components.BadgeProps](components.ColorPrimary, false),
					components.WithBadgeDescription(strconv.Itoa(count)),
				)),
			content: feed.GetContent(),
			buttons: []templ.Component{
				buttonToggleItem(feed.GetID(), ""),
				buttonShareItem(feed.GetID(), ""),
			},
		},
		nil
}
