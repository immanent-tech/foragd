// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/handlers/renderers"
)

func feedNameInput() components.Input {
	return components.NewInput("Name",
		components.AsFormControl(),
		components.OptionalInput(),
		components.WithInputLabel("Name"),
		components.WithPlaceholder("The Feed"),
		components.WithInputAttributes(templ.Attributes{
			"hx-post": "/subscription/validate",
		}),
	)
}

func feedURLInput() components.Input {
	return components.NewInput("Link",
		components.AsFormControl(),
		components.WithInputLabel("Link"),
		components.WithPlaceholder("https://my.favourite.site/feed.rss"),
		components.WithInputAttributes(templ.Attributes{
			"hx-post": "/subscription/validate",
		}),
	)
}

func feedTopicsInput() components.Input {
	return components.NewInput("Topics",
		components.AsFormControl(),
		components.OptionalInput(),
		components.WithInputLabel("Topics"),
		components.WithPlaceholder("CoolStuff, Memes"),
		components.WithInputAttributes(templ.Attributes{
			"hx-post": "/subscription/validate",
		}),
	)
}

func feedItemForm(inputs ...components.Input) components.Form {
	return components.NewForm("addItem",
		components.Info("Enter Feed Details."),
		components.FormAttributes(templ.Attributes{
			"hx-post": "/subscription/add",
		}),
		components.Inputs(inputs...),
		components.Buttons(
			components.NewButton("Save", "save",
				components.ButtonAttributes(templ.Attributes{
					"_": "on click take .modal-open from #command-modal wait 200ms",
				}),
			),
		),
	)
}

// AddItem is the handler for adding a new feed (GET /home/add).
func AddItem(res http.ResponseWriter, req *http.Request) {
	addItemForm := feedItemForm(feedNameInput(), feedURLInput(), feedTopicsInput())

	if err := renderers.CommandModal(req, res, addItemForm.Show()); err != nil {
		logging.FromContext(req.Context()).
			Warn("Unable to command modal.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func ProcessAddItem(res http.ResponseWriter, req *http.Request, storeAPI dataStore) {
	item, problems, err := decodeForm[*models.SubscriptionRequest](req)
	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode submitted add feed request.", slog.Any("error", err))
		Validate(res, req, UpdateAddItemForm)
		return
	}

	if err := storeAPI.AddSubscription(req.Context(), item); err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}
}

// UpdateAddItemForm takes the user input, validation results and decorates the
// add item form with the results.
func UpdateAddItemForm(field string, item *models.SubscriptionRequest, problems models.ValidationErrors) components.Input {
	form := feedItemForm(feedNameInput(), feedURLInput(), feedTopicsInput())

	input, _ := form.Inputs.Get(field)

	switch field {
	case "Name":
		input.SetValue(item.Name)
	case "Link":
		input.SetValue(item.Link)
	case "Topics":
		input.SetValue(item.Topics)
	}

	if issue, found := problems[field]; found {
		input.Error = issue
	}

	return input
}
