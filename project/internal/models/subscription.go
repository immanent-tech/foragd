// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"strings"

	"github.com/a-h/templ"
	"github.com/davecgh/go-spew/spew"
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
		components.WithInsideLabels(
			components.PlainText("Nickname"),
			components.HelperDropdown(
				"Optional. Replace the name of the feed with your own custom nickname.",
				components.WithOpenFrom[components.DropdownProps](components.OpenLeft),
			),
		),
		components.WithID[components.TextInputProps]("name"),
		components.WithPlaceholder[components.TextInputProps]("Cool feed"),
	)
	if s.Name != "" {
		s.InputName.SetValue(s.Name)
	}

	s.InputURL = components.BuildTextInput(
		components.WithFormControl(),
		components.WithInsideLabels(
			components.PlainText("URL"),
			components.HelperDropdown(
				"The URL for the feed.",
				components.WithOpenFrom[components.DropdownProps](components.OpenLeft),
			),
		),
		components.WithID[components.TextInputProps]("url"),
		components.AsURL(),
		components.WithColor[components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[components.TextInputProps]("https://cool.feed.lol/rss"),
		components.WithValidationRequired[components.TextInputProps](),
	)
	if s.URL != "" {
		s.InputURL.SetValue(s.URL)
	}

	s.InputCategories = components.BuildTextInput(
		components.WithFormControl(),
		components.WithInsideLabels(
			components.PlainText("Categories"),
			components.HelperDropdown(
				"Optional. A (comma-separated) list of custom categories to group this feed with others.",
				components.WithOpenFrom[components.DropdownProps](components.OpenLeft),
			),
		),
		components.WithID[components.TextInputProps]("categories"),
		components.WithColor[components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[components.TextInputProps]("awesome, news"),
	)
	if len(s.Categories) > 0 {
		s.InputCategories.SetValue(strings.Join(s.Categories, ","))
	}
}

func (s *SubscriptionRequest) Form() templ.Component {
	return components.Form(
		components.WithFormComponents(
			components.TextInput(
				components.FromTextInputProps(s.InputName),
			),
			components.TextInput(
				components.FromTextInputProps(s.InputURL),
			),
			components.TextInput(
				components.FromTextInputProps(s.InputCategories),
			),
			components.Button(
				components.WithID[components.ButtonProps]("add"),
				components.WithButtonContent(components.AsTextContent("Add")),
				components.WithColor[components.ButtonProps](components.ColorPrimary, true),
			),
		),
		components.WithAttributes[components.FormProps](
			templ.Attributes{
				"hx-post":   "/home/subscription/add",
				"hx-target": "#command_modal",
			},
		),
	)
}

func (s *SubscriptionRequest) Valid() bool {
	valid, problems := validateStruct(s)
	if valid {
		return true
	}

	spew.Dump(problems)

	s.generateInputs()

	for field, problem := range problems {
		switch field {
		case "URL":
			s.InputURL.SetStateColor(components.StateError, true)
			s.InputURL.SetBottomRightLabel(problem)
		}
	}

	return false
}
