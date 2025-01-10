// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
)

var ErrAddUser = errors.New("add subscription failed")

type UserPreferences map[string]any

func NewUserPreferences() UserPreferences {
	return map[string]any{
		"theme": "light",
	}
}

func (u *User) GetSubscribedFeedIDs() []FeedID {
	feedIDs := make([]FeedID, len(u.Subscriptions))
	idx := 0

	for feedID := range u.Subscriptions {
		feedIDs[idx] = feedID
		idx++
	}

	return feedIDs
}

func (u *User) GetReadItemIDs(feedIDs ...FeedID) []ItemID {
	var readItemsIDs []ItemID

	for feedID, items := range u.ReadItems {
		if len(feedIDs) > 0 {
			if !slices.Contains(feedIDs, feedID) {
				continue
			}
		}

		for _, item := range items {
			readItemsIDs = append(readItemsIDs, item.ItemID)
		}
	}

	return readItemsIDs
}

func (u *User) DocumentID() *string {
	return &u.ID
}

func (u *User) DocumentType() DocumentType {
	return TypeUser
}

func (u *User) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(u)
}

func (t *Tokens) UserID() string {
	return t.IDToken.Subject
}

func (t *Tokens) Nickname() string {
	return t.Claims.UserNickName
}

func (t *Tokens) Email() string {
	return t.Claims.UserName
}

func (t *Tokens) DecodeClaims() error {
	var claims Claims

	if err := t.IDToken.Claims(&claims); err != nil {
		return fmt.Errorf("cannot decode user claims from ID token: %w", err)
	}

	t.Claims = claims

	return nil
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

func (s *UserSignup) ShowNickNameInput() templ.Component {
	return components.TextInput(
		components.FromTextInputProps(s.InputNickname),
	)
}

func (s *UserSignup) ShowEmailInput() templ.Component {
	return components.TextInput(
		components.FromTextInputProps(s.InputEmail),
	)
}

func (s *UserSignup) ShowPasswordInput() templ.Component {
	return components.TextInput(
		components.FromTextInputProps(s.InputPassword),
	)
}
