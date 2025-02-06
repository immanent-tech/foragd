// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"

	"github.com/a-h/templ"
	"github.com/davecgh/go-spew/spew"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/validation"
)

var ErrNewSubscriptionRequest = errors.New("could not create new subscription request")

func (r *APISubscriptionRequest) Valid() bool {
	valid, problems := validation.ValidateStruct(r)
	if valid {
		return true
	}

	spew.Dump(r)

	if len(r.ValidationErrors) == 0 {
		r.ValidationErrors = make(map[string]string)
	}

	for field, problem := range problems {
		r.ValidationErrors[field] = problem
	}

	return false
}

func (r *APISubscriptionRequest) HasErrors() bool {
	return r.ValidationErrors != nil
}

func (r *APISubscriptionRequest) GenerateNameInput() templ.Component {
	input := components.BuildTextInput(
		components.WithFormControl(),
		components.WithResponsiveSize[*components.TextInputProps](components.SM),
		components.WithInsideLabels(
			components.Text("Nickname", components.WithTextSize(components.TextSM)),
			components.HelperDropdown(
				"Optional. Replace the name of the feed with your own custom nickname.",
				components.WithOpenFrom[*components.DropdownProps](components.OpenLeft),
				components.WithOpenOnHover(),
			),
		),
		components.WithName[*components.TextInputProps]("name"),
		components.WithPlaceholder[*components.TextInputProps]("Cool feed"),
		components.WithAttributes[*components.TextInputProps](templ.Attributes{
			"onkeydown": "if (event.keyCode === 13) event.preventDefault();",
		}),
	)
	// Check for validation or input errors and mark the field with error status.
	if r.Name != nil {
		if r.ValidationErrors["Name"] != "" {
			input.SetBottomRightLabel(r.ValidationErrors["Name"])
		}

		input.SetValue(*r.Name)
	}

	return components.TextInput(components.FromTextInputProps(input))
}

// subscriptionURL generates the text input for the subscription URL.
func (r *APISubscriptionRequest) GenerateURLInput() templ.Component {
	input := components.BuildTextInput(
		components.WithFormControl(),
		components.WithResponsiveSize[*components.TextInputProps](components.SM),
		components.WithInsideLabels(
			components.Text("URL", components.WithTextSize(components.TextSM)),
			components.HelperDropdown(
				"The URL for the feed.",
				components.WithOpenFrom[*components.DropdownProps](components.OpenLeft),
				components.WithOpenOnHover(),
			),
		),
		components.WithName[*components.TextInputProps]("url"),
		components.AsURL(),
		components.WithColor[*components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[*components.TextInputProps]("https://cool.feed.lol/rss"),
		components.WithValidationRequired[*components.TextInputProps](),
		components.WithAttributes[*components.TextInputProps](templ.Attributes{
			"onkeydown": "if (event.keyCode === 13) event.preventDefault();",
			// "onkeyup":                        "this.setCustomValidity('')",
			// "hx-on:htmx:validation:validate": "/feed/parse_url",
		}),
	)
	// Check for validation errors and mark the field with error status.
	if r.URL != "" {
		input.SetValue(r.URL)

		if r.ValidationErrors["URL"] != "" {
			input.SetColor(components.ColorStateError, true)
			input.SetBottomRightLabel(r.ValidationErrors["URL"])
		}
	}

	return components.TextInput(components.FromTextInputProps(input))
}

func (r *APISubscriptionRequest) GenerateCategoriesList() templ.Component {
	categories := make([]templ.Component, 0, len(r.Categories))

	for _, category := range r.Categories {
		categories = append(categories,
			GenerateCategoryItem(category),
		)
	}

	// Categories in an unordered list.
	return components.UnorderedList(
		components.WithID[*components.UnorderedListProps]("categories"),
		components.WithItems[*components.UnorderedListProps](categories...),
		components.WithAttributes[*components.UnorderedListProps](templ.Attributes{
			"hx-target": "closest li",
			"hx-swap":   "outerHTML swap:1s",
		}),
	)
}
