// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spaolacci/murmur3"
	"github.com/stripe/stripe-go/v83"

	"github.com/immanent-tech/foragd/validation"
)

const (
	// DefaultUserTheme is the default theme for the app.
	DefaultUserTheme = "greenhouse"

	GathererMaxHistory        = 7 * 24 * time.Hour // One week.
	GathererUpdatesFrequency  = time.Hour
	CollectorMaxHistory       = 30 * 24 * time.Hour // One month.
	CollectorUpdatesFrequency = 5 * time.Minute
	CuratorMaxHistory         = 365 * 24 * time.Hour // One year.
	CuratorUpdatesFrequency   = time.Minute
)

var (
	ErrUserNotSubscribed    = errors.New("not subscribed")
	ErrUserAlreadyFavorited = errors.New("already a favorite")
)

// NewUser creates a new user from the external provider details.
func NewUser(externalID, email string) *User {
	ts := time.Now().UTC()
	user := &User{
		CreatedAt:      ts,
		UpdatedAt:      ts,
		ExternalUserID: externalID,
		Provider:       strings.Split(externalID, "|")[0],
		Email:          email,
		UserID:         strings.Join([]string{"user_", strconv.FormatUint(murmur3.Sum64([]byte(externalID)), 10)}, "_"),
		Settings: UserSettings{
			Theme:                 DefaultUserTheme,
			ShowOnboarding:        true,
			ShowSubscriptionStats: false,
			MarkArticleReadOnView: true,
		},
	}

	return user
}

// Valid returns a boolean indicating whether the user data is valid. If not valid, it will also return a non-nil error
// that contains the validation issues.
func (u *User) Valid(_ context.Context) error {
	if err := validation.Validate.Struct(u); err != nil {
		return fmt.Errorf("user data is invalid: %w", err)
	}
	return nil
}

// GetID returns the ID for the user.
func (u *User) GetID() UserID {
	return u.UserID
}

// GetExternalID returns the backend ID for the user.
func (u *User) GetExternalID() UserID {
	return u.ExternalUserID
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

// GetMaxHistory returns a timestamp in the past from which the user can view items. If there is an issue retrieving and
// parsing the value from the user's metadata, it will default to using the lowest plan max history.
func (u *User) GetMaxHistory() time.Time {
	if u.Metadata.MaxHistory == "" {
		return time.Now().Add(-GathererMaxHistory)
	}

	dur, err := time.ParseDuration(u.Metadata.MaxHistory)
	if err != nil {
		return time.Now().Add(-GathererMaxHistory)
	}

	return time.Now().Add(-dur)
}

// GetUpdatesFrequency returns a duration on which the user will see new updates. If there is an issue retrieving and
// parsing the value from the user's metdata, it will use the lowest plan updates frequency.
func (u *User) GetUpdatesFrequency() time.Duration {
	freq, err := time.ParseDuration(u.Metadata.UpdatesFrequency)
	if err != nil {
		return GathererUpdatesFrequency
	}
	return freq
}

// OnTrial returns a boolean indicating whether the user is currently in a trial period and if so, a timestamp
// indicating when the trial will end.
func (u *User) OnTrial() (bool, time.Time) {
	if u.Metadata.PlanStatus == stripe.SubscriptionStatusTrialing {
		return true, u.Metadata.TrialEnd
	}
	return false, time.Time{}
}

// Cancelled returns a boolean indicating whether the user has cancelled their subscription plan and if so, a timestamp
// indicating when the cancellation will apply.
func (u *User) Cancelled() (bool, time.Time) {
	if u.Metadata.PlanStatus == stripe.SubscriptionStatusCanceled || u.Metadata.CancelAt.After(time.Now().UTC()) {
		return true, u.Metadata.CancelAt
	}
	return false, time.Time{}
}

// Active returns a boolean indicating whether the user is "active", which means a paying customer with no payment
// issues or customer currently on a trial.
func (u *User) Active() bool {
	if u.Metadata.PlanStatus == stripe.SubscriptionStatusActive {
		return true
	}
	if trial, _ := u.OnTrial(); trial {
		return true
	}
	return false
}

// GetSubscriptionPlan returns the name of the subscription plan of the user.
func (u *User) GetSubscriptionPlan() string {
	return u.Metadata.Plan
}

// GetSettings returns the user's settings. If the user has no settings (i.e. new user), default settings will be
// returned.
func (u *User) GetSettings() *UserSettings {
	return &u.Settings
}

// GetPlan returns the subscription level the user has paid for. If this is missing, it defaults to the
// lowest level (gatherer) subscription.
func (u *User) GetPlan() string {
	return u.Metadata.Plan
}

// Valid returns a boolean indicating if the UserSettings contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *UserSettings) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("invalid user settings: %w", err)
	}
	return nil
}

// Sanitise will sanitise UserSettings values.
func (s *UserSettings) Sanitise() error {
	return nil
}

// Valid returns a boolean indicating if the UserSettings contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *UserMetadata) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("invalid user metadata: %w", err)
	}
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
	if err := validation.Validate.Struct(s); err != nil {
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
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the user input for a ChangePasswordRequest.
func (r *ChangePasswordRequest) Sanitise() error {
	return nil
}
