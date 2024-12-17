// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package partials

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/models"
)

func SignUpForm() components.Form {
	return components.NewForm("signUp",
		components.Info("Enter your details to create a new account."),
		components.FormAttributes(templ.Attributes{
			"hx-post": "/signup",
		}),
		components.Inputs(
			components.NewInput(
				components.WithID[components.Input]("Nickname"),
				components.AsFormControl(),
				components.OptionalInput(),
				components.WithInputLabel("Nickname"),
				components.WithPlaceholder("SomeCoolName"),
				components.WithAttributes[components.Input](templ.Attributes{
					"hx-post": "/signup/validate",
				}),
			),
			components.NewInput(
				components.WithID[components.Input]("Email"),
				components.AsFormControl(),
				components.WithInputLabel("Email"),
				components.WithPlaceholder("you@yourdomain.com"),
				components.WithAttributes[components.Input](templ.Attributes{
					"hx-post": "/signup/validate",
				}),
			),
			components.NewInput(
				components.WithID[components.Input]("Password"),
				components.AsFormControl(),
				components.WithInputType(components.InputTypePassword),
				components.WithInputLabel("Password"),
				components.WithAttributes[components.Input](templ.Attributes{
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

// UpdateSignupInput takes the user input, validation results and decorates the
// sign up form with the results.
func UpdateSignupInput(field string, item *models.APIUser, problems models.ValidationErrors) components.Input {
	form := SignUpForm()

	input, _ := form.Inputs.Get(field)

	switch field {
	case "Email":
		input.SetAttribute("value", item.Email)
	case "Password":
		input.SetAttribute("value", item.Password)
	case "Nickname":
		input.SetAttribute("value", item.Nickname)
	}

	if issue, found := problems[field]; found {
		input.Error = issue
	}

	return input
}
