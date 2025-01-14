// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
)

// NewUserSignup creates a new UserSignup request.
func NewUserSignup() *UserSignup {
	s := &UserSignup{}
	s.generateInputs()

	return s
}

func (s *UserSignup) generateInputs() {
	s.InputNickname = components.BuildTextInput(
		components.WithFormControl(),
		components.WithInsideLabels(
			components.Icon("fa-user"),
			components.Badge(
				components.WithBadgeDescription("Optional"),
				components.WithColor[components.BadgeProps](components.ColorPrimary, true))),
		components.WithID[components.TextInputProps]("nickname"),
		components.WithPlaceholder[components.TextInputProps]("Your name"),
	)
	if s.Nickname != "" {
		s.InputNickname.SetValue(s.Nickname)
	}

	s.InputEmail = components.BuildTextInput(
		components.WithFormControl(),
		components.WithInsideLabels(
			components.Icon("fa-at"),
			nil),
		components.WithID[components.TextInputProps]("email"),
		components.AsEmail(),
		components.WithColor[components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[components.TextInputProps]("youremail@domain.com"),
		components.WithValidationRequired[components.TextInputProps](),
	)
	if s.Email != "" {
		s.InputEmail.SetValue(s.Email)
	}

	s.InputPassword = components.BuildTextInput(
		components.WithFormControl(),
		components.WithInsideLabels(
			components.Icon("fa-key"),
			nil),
		components.WithID[components.TextInputProps]("password"),
		components.AsPassword(),
		components.WithColor[components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[components.TextInputProps]("supersecret"),
	)
	if s.Password != "" {
		s.InputPassword.SetValue(s.Password)
	}
}

func (s *UserSignup) Form() templ.Component {
	return components.Form(
		components.WithFormComponents(
			components.TextInput(
				components.FromTextInputProps(s.InputNickname),
			),
			components.TextInput(
				components.FromTextInputProps(s.InputEmail),
			),
			components.TextInput(
				components.FromTextInputProps(s.InputPassword),
			),
			components.Button(
				components.NewButton("Signup", "signup",
					components.WithColor[components.Button](components.ColorPrimary, true)),
			).Show(),
		),
		components.WithAttributes[components.FormProps](
			templ.Attributes{
				"hx-post":   "/signup",
				"hx-target": "#signup",
			},
		),
	)
}

func (s *UserSignup) Valid() bool {
	valid, problems := validateStruct(s)
	if valid {
		return true
	}

	s.generateInputs()

	for field, problem := range problems {
		switch field {
		case "Email":
			s.InputEmail.SetState(components.StateError, true)
			s.InputEmail.SetBottomRightLabel(problem)
		case "Password":
			s.InputPassword.SetState(components.StateError, true)
			s.InputPassword.SetBottomRightLabel(problem)
		}
	}

	return false
}
