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
	"github.com/joshuar/go-feed-me/internal/server/session"
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

func addSubscriptionForm(inputs ...components.Input) components.Form {
	return components.NewForm("addItem",
		components.Info("Enter Details."),
		components.FormAttributes(templ.Attributes{
			"hx-post": "/subscription/add",
		}),
		components.Inputs(inputs...),
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

// AddSubscriptionHandler is the handler for adding a new feed (GET /home/add).
func AddSubscriptionHandler(res http.ResponseWriter, req *http.Request) {
	addItemForm := addSubscriptionForm(subscriptionNameInput(), subscriptionLinkInput())

	if err := renderers.CommandModal(req, res, addItemForm.Show()); err != nil {
		logging.FromContext(req.Context()).
			Warn("Unable to command modal.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func ProcessAddSubscriptionForm(res http.ResponseWriter, req *http.Request, cache models.Cache, db models.DB) {
	newSubscription, problems, err := DecodeForm[*models.APISubscription](req)
	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode submitted add feed request.", slog.Any("error", err))
		Validate(res, req, UpdateAddSubscriptionForm)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	userID, err := session.UserID(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not retrieve user ID from session.")
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := models.AddNewSubscription(req.Context(), userID, cache, db, newSubscription); err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}

// UpdateAddSubscriptionForm takes the user input, validation results and decorates the
// add item form with the results.
func UpdateAddSubscriptionForm(field string, item *models.APISubscription, problems models.ValidationErrors) components.Input {
	form := addSubscriptionForm(subscriptionNameInput(), subscriptionLinkInput())

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
