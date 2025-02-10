// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/display/icon"
	"github.com/joshuar/go-templ-daisyui/input/form"
	"github.com/joshuar/go-templ-daisyui/modifiers/color"

	"github.com/joshuar/go-feed-me/internal/validation"
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
			icon.Build("fa-user").Show(),
			nil),
		components.WithID[*components.TextInputProps]("nickname"),
		components.WithPlaceholder[*components.TextInputProps]("Your name"),
	)
	if s.Nickname != "" {
		s.InputNickname.SetValue(s.Nickname)
	}

	s.InputEmail = components.BuildTextInput(
		components.WithFormControl(),
		components.WithInsideLabels(
			icon.Build("fa-at").Show(),
			nil),
		components.WithID[*components.TextInputProps]("email"),
		components.AsEmail(),
		components.WithColor[*components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[*components.TextInputProps]("youremail@domain.com"),
		components.WithValidationRequired[*components.TextInputProps](),
	)
	if s.Email != "" {
		s.InputEmail.SetValue(s.Email)
	}

	s.InputPassword = components.BuildTextInput(
		components.WithFormControl(),
		components.WithInsideLabels(
			icon.Build("fa-key").Show(),
			nil),
		components.WithID[*components.TextInputProps]("password"),
		components.AsPassword(),
		components.WithColor[*components.TextInputProps](components.ColorPrimary, false),
		components.WithPlaceholder[*components.TextInputProps]("supersecret"),
	)
	if s.Password != "" {
		s.InputPassword.SetValue(s.Password)
	}
}

func (s *UserSignup) Form() templ.Component {
	return form.BuildForm(
		form.WithExtraAttributes(
			templ.Attributes{
				"hx-post":   "/signup",
				"hx-target": "#signup",
			},
		),
		form.WithElements(
			components.TextInput(
				components.FromTextInputProps(s.InputNickname),
			),
			components.TextInput(
				components.FromTextInputProps(s.InputEmail),
			),
			components.TextInput(
				components.FromTextInputProps(s.InputPassword),
			),
			button.Build(
				button.WithID("signup"),
				button.WithContent("Signup"),
				button.WithThemeColor(color.Primary, true),
			).Show(),
		),
	).Show()
}

func (s *UserSignup) Valid() bool {
	valid, problems := validation.ValidateStruct(s)
	if valid {
		return true
	}

	s.generateInputs()

	for field, problem := range problems {
		switch field {
		case "Email":
			s.InputEmail.SetColor(components.ColorStateError, true)
			s.InputEmail.SetBottomRightLabel(problem)
		case "Password":
			s.InputPassword.SetColor(components.ColorStateError, true)
			s.InputPassword.SetBottomRightLabel(problem)
		}
	}

	return false
}
