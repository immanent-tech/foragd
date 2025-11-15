// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/immanent-tech/foragd/validation"
)

const (
	// DefaultUserTheme is the default theme for the app.
	DefaultUserTheme = "garden"

	BasicAccountMaxHistory          = 7 * 24 * time.Hour // One week.
	BasicAccountUpdatesFrequency    = time.Hour
	StandardAccountMaxHistory       = 30 * 24 * time.Hour // One month.
	StandardAccountUpdatesFrequency = 5 * time.Minute
	PremiumAccountMaxHistory        = 365 * 24 * time.Hour // One year.
	PremiumAccountUpdatesFrequency  = time.Minute
)

var (
	ErrUserNotSubscribed    = errors.New("not subscribed")
	ErrUserAlreadyFavorited = errors.New("already a favorite")
)

// NewUser creates a new user from the external provider details.
func NewUser(externalID, email, provider string, level UserLevel) *User {
	ts := time.Now().UTC()
	user := &User{
		CreatedAt:      ts,
		UpdatedAt:      ts,
		ExternalUserId: externalID,
		Email:          email,
		Provider:       provider,
		UserID:         NewID(UserPFX),
		Settings: UserSettings{
			Theme:                 DefaultUserTheme,
			ShowOnboarding:        true,
			ShowSubscriptionStats: false,
			MarkArticleReadOnView: true,
		},
		Level: level,
	}
	// Set account level based user settings.
	switch user.Level {
	case UserLevelBasic:
		user.Settings.MaxHistory = BasicAccountMaxHistory.String()
		user.Settings.UpdatesFrequency = BasicAccountUpdatesFrequency.String()
	case UserLevelStandard:
		user.Settings.MaxHistory = StandardAccountMaxHistory.String()
		user.Settings.UpdatesFrequency = StandardAccountUpdatesFrequency.String()
	case UserLevelCustom, UserLevelPremium:
		user.Settings.MaxHistory = PremiumAccountMaxHistory.String()
		user.Settings.UpdatesFrequency = PremiumAccountUpdatesFrequency.String()
	}
	return user
}

// Valid returns a boolean indicating whether the user data is valid. If not valid, it will also return a non-nil error
// that contains the validation issues.
func (u *User) Valid(_ context.Context) (bool, error) {
	err := validation.Validate.Struct(u)
	if err != nil {
		return false, fmt.Errorf("user data is invalid: %w", err)
	}
	return true, nil
}

// GetID returns the ID for the user.
func (u *User) GetID() UserID {
	return u.UserID
}

// GetAvatar retrieves the URL to the image to represent the user.
func (u *User) GetAvatar() string {
	return u.AvatarURL
}

// GetNickname retrieves the nickname of the user.
func (u *User) GetNickname() string {
	return u.Nickname
}

// GetEmail retrieves the email of the user.
func (u *User) GetEmail() string {
	return u.Email
}

// GetMaxHistory returns a timestamp in the past from which the user can view
// items.
func (u *User) GetMaxHistory() time.Time {
	return parseMaxHistory(u.GetSettings().MaxHistory)
}

// GetSettings returns the user's settings. If the user has no settings (i.e. new user), default settings will be
// returned.
func (u *User) GetSettings() *UserSettings {
	return &u.Settings
}

// Valid returns a boolean indicating if the UserSettings contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *UserSettings) Valid() error {
	err := validation.Validate.Struct(s)
	if err != nil {
		return fmt.Errorf("invalid user settings: %w", err)
	}
	return nil
}

// Sanitise will sanitise UserSettings values.
func (s *UserSettings) Sanitise() error {
	return nil
}

// GetUserTheme returns the current user's theme or the default theme if no user theme is set.
func GetUserTheme(ctx context.Context) string {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return DefaultUserTheme
	}
	if theme := user.GetSettings().Theme; theme != "" {
		return theme
	}
	return DefaultUserTheme
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *EditUserRequest) Valid() error {
	err := validation.Validate.Struct(s)
	if err != nil {
		return fmt.Errorf("request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the user input for a SubscriptionCustomisation.
func (s *EditUserRequest) Sanitise() error {
	s.Nickname = validation.SanitizeString(s.Nickname)
	return nil
}

// Valid returns a boolean indicating whether the ChangePasswordRequest contains valid data.
func (r *ChangePasswordRequest) Valid() error {
	err := validation.Validate.Struct(r)
	if err != nil {
		return fmt.Errorf("request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the user input for a ChangePasswordRequest.
func (r *ChangePasswordRequest) Sanitise() error {
	return nil
}
