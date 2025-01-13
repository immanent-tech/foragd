// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"strings"

	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
)

// NewUserSignup creates a new UserSignup request.
func NewSubscriptionAddRequest() *SubscriptionRequest {
	s := &SubscriptionRequest{}
	s.generateInputs()

	return s
}

func (s *SubscriptionRequest) generateInputs() {
	s.InputName = components.BuildTextInput(
		components.WithFormControl(),
		components.WithOutsideLabels("Name", "", "Replaces the feed's own name.", ""),
		components.WithInsideLabels(
			components.Icon("fa-rss"),
			components.Badge(
				components.WithBadgeDescription("Optional"),
				components.WithColor[components.BadgeProps](components.ColorPrimary, true))),
		components.WithID[components.TextInputProps]("name"),
		components.WithPlaceholder[components.TextInputProps]("Your custom name for this feed"),
	)
	if s.Name != "" {
		s.InputName.SetValue(s.Name)
	}

	s.InputURL = components.BuildTextInput(
		components.WithFormControl(),
		components.WithOutsideLabels("URL", "", "The URL for the feed.", ""),
		components.WithInsideLabels(
			components.Icon("fa-link"),
			nil),
		components.WithID[components.TextInputProps]("url"),
		components.AsURL(),
		components.WithColor[components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[components.TextInputProps]("http://awesome.feed/rss"),
		components.WithValidationRequired[components.TextInputProps](),
	)
	if s.URL != "" {
		s.InputURL.SetValue(s.URL)
	}

	s.InputCategories = components.BuildTextInput(
		components.WithFormControl(),
		components.WithOutsideLabels("Categories", "", "A list of custom categories to group this feed with others.", ""),
		components.WithInsideLabels(
			components.Icon("fa-list"),
			nil),
		components.WithID[components.TextInputProps]("categories"),
		components.WithColor[components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[components.TextInputProps]("feeds, stuff, news"),
	)
	if len(s.Categories) > 0 {
		s.InputCategories.SetValue(strings.Join(s.Categories, ","))
	}
}

func (s *SubscriptionRequest) ShowNameInput() templ.Component {
	return components.TextInput(
		components.FromTextInputProps(s.InputName),
	)
}

func (s *SubscriptionRequest) ShowURLInput() templ.Component {
	return components.TextInput(
		components.FromTextInputProps(s.InputURL),
	)
}

func (s *SubscriptionRequest) ShowCategoriesInput() templ.Component {
	return components.TextInput(
		components.FromTextInputProps(s.InputCategories),
	)
}
