// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/web/templates"
)

// Render will render a Component appropriately based on its DisplayType.
func Render(component *templates.Component) (templ.Component, error) {
	switch component.DisplayType {
	case templates.FeedCard:
		return ShowFeedCard(component)
	case templates.ItemCard:
		return ShowItemCard(component)
	case templates.ItemArticle:
		return ShowArticle(component)
	}

	return nil, templates.ErrDisplayComponent
}
