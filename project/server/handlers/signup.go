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
	"github.com/angelofallars/htmx-go"

	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/model"
	"github.com/joshuar/go-feed-me/templates/partials"
)

func signUpForm() components.Form {
	return components.NewForm("signUp",
		components.Info("Enter your details to create a new account."),
		components.FormAttributes(templ.Attributes{
			"hx-post": "/signup",
		}),
		components.Inputs(
			components.NewInput("Nickname",
				components.AsFormControl(),
				components.OptionalInput(),
				components.WithInputLabel("Nickname"),
				components.WithPlaceholder("SomeCoolName"),
				components.WithInputAttributes(templ.Attributes{
					"hx-post": "/signup/validate",
				}),
			),
			components.NewInput("Email",
				components.AsFormControl(),
				components.WithInputLabel("Email"),
				components.WithPlaceholder("you@yourdomain.com"),
				components.WithInputAttributes(templ.Attributes{
					"hx-post": "/signup/validate",
				}),
			),
			components.NewInput("Password",
				components.AsFormControl(),
				components.WithInputType(components.InputTypePassword),
				components.WithInputLabel("Password"),
				components.WithInputAttributes(templ.Attributes{
					"hx-post": "/signup/validate",
				}),
			),
		),
		components.Buttons(
			components.NewButton("Signup", "signup",
				components.WithModifier(components.ButtonAccent)),
		),
	)
}

// Signup will display a sign up form.
func Signup(res http.ResponseWriter, req *http.Request) {
	form := signUpForm()

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignUpForm(form)); err != nil {
		logging.FromContext(req.Context()).
			Error("Cannot render signup form.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// ProcessSignup takes the validated user sign up values and creates a new user.
func ProcessSignup(res http.ResponseWriter, req *http.Request, userAPI authAPI, storeAPI storeAPI) {
	item, problems, err := decodeForm[*model.UserSignup](req)
	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode submitted signup request.", slog.Any("error", err))
		Validate(res, req, UpdateSignupInput)
		return
	}

	// Create the user in the auth backend.
	user, err := userAPI.Create(req.Context(), item)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not create user account.", slog.Any("error", err))

		if err = htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupError()); err != nil {
			logging.FromContext(req.Context()).
				Error("Cannot render sign up error.", slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
		}

		return
	}

	// Add the new user to the store.
	err = storeAPI.AddUser(req.Context(), user)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not create user account.", slog.Any("error", err))

		if err = htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupError()); err != nil {
			logging.FromContext(req.Context()).
				Error("Cannot render sign up error.", slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
		}
	}

	// Show success message.
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, partials.SignupSuccess()); err != nil {
		logging.FromContext(req.Context()).
			Error("Cannot render sign up error.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

// UpdateSignupInput takes the user input, validation results and decorates the
// sign up form with the results.
func UpdateSignupInput(field string, item *model.UserSignup, problems model.ValidationErrors) components.Input {
	form := signUpForm()

	input, _ := form.Inputs.Get(field)

	switch field {
	case "Email":
		input.Attributes["value"] = item.Email
	case "Password":
		input.Attributes["value"] = item.Password
	case "Nickname":
		input.Attributes["value"] = item.Nickname
	}

	if issue, found := problems[field]; found {
		input.Error = issue
	}

	return input
}
