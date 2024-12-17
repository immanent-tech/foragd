// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/models"
)

func subscriptionNameInput() components.Input {
	return components.NewInput(
		components.WithID[components.Input]("Name"),
		components.AsFormControl(),
		components.OptionalInput(),
		components.WithInputLabel("Name"),
		components.WithPlaceholder("The Feed"),
		components.WithAttributes[components.Input](templ.Attributes{
			"hx-post": "/subscription/validate",
		}),
	)
}

func subscriptionLinkInput() components.Input {
	return components.NewInput(
		components.WithID[components.Input]("URL"),
		components.AsFormControl(),
		components.WithInputLabel("Link"),
		components.WithPlaceholder("https://my.favourite.site/feed.rss"),
		components.WithAttributes[components.Input](templ.Attributes{
			"hx-post": "/subscription/validate",
		}),
	)
}

func AddSubscriptionForm() components.Form {
	return components.NewForm("addItem",
		components.Info("Enter Details."),
		components.FormAttributes(templ.Attributes{
			"hx-post": "/subscription/add",
		}),
		components.Inputs(subscriptionNameInput(), subscriptionLinkInput()),
		components.Buttons(
			components.NewButton("Save", "save",
				components.WithSize[components.Button](components.LG),
				components.WithAttributes[components.Button](templ.Attributes{
					"_": "on click take .modal-open from #command-modal wait 200ms",
				}),
			),
		),
	)
}

// UpdateAddSubscriptionForm takes the user input, validation results and decorates the
// add item form with the results.
func UpdateAddSubscriptionForm(field string, item *models.APISubscription, problems models.ValidationErrors) components.Input {
	form := AddSubscriptionForm()

	input, _ := form.Inputs.Get(field)

	switch field {
	case "Name":
		input.SetValue(item.Name)
	case "URL":
		input.SetValue(item.URL)
	}

	if issue, found := problems[field]; found {
		input.Error = issue
	}

	return input
}
