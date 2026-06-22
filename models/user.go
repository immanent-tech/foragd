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
	// DefaultMaxHistory is a default maximum history value for when the user has not specified one.
	DefaultMaxHistory = 31 * 24 * time.Hour
	// DefaultUpdateInterval is the default interval on which to check for updates.
	DefaultUpdateInterval = 5 * time.Minute
	// MaxSubscriptions is the maximum number of subscriptions a user can have.
	MaxSubscriptions = 3000
	// MaxEmailNewsletters is the maximum number of email newsletter subscriptions a user can have.
	MaxEmailNewsletters = 50
	// LimitExceededGracePeriod is the grace period in which the user is allowed to remain over an account limit.
	LimitExceededGracePeriod = 7 * 24 * time.Hour
	// DefaultTrialPeriod is the default amount of time a trial runs for.
	DefaultTrialPeriod = 14 * 24 * time.Hour
)

var (
	ErrUserAlreadyFavorited = errors.New("already a favorite")
)

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
	if u.AvatarURL != nil {
		return *u.AvatarURL
	}
	return ""
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
	if u.GetSettings().MaxViewHistory == 0 {
		return time.Now().Add(-DefaultMaxHistory)
	}

	return time.Now().Add(-u.GetSettings().MaxViewHistory)
}

// GetUpdatesFrequency returns a duration on which the user will see new updates. If there is an issue retrieving and
// parsing the value from the user's metadata, it will use the lowest plan updates frequency.
func (u *User) GetUpdatesFrequency() time.Duration {
	if u.GetSettings().UpdatesInterval == 0 {
		return DefaultUpdateInterval
	}
	return u.GetSettings().UpdatesInterval
}

// GetSettings returns the user's settings. If the user has no settings (i.e. new user), default settings will be
// returned.
func (u *User) GetSettings() *UserSettings {
	return &u.Settings
}

// InTrial returns a boolean indicating whether this user account is in its trial period.
func (u *User) InTrial() bool {
	if !u.HasValidSubscription() {
		// No current subscription, check against account creation time.
		if time.Now().UTC().Before(u.CreatedAt.Add(DefaultTrialPeriod)) {
			return true
		}
	}
	return false
}

// InTrialGracePeriod returns a boolean indicating whether the user account is in its trial grace period. The trial
// grace period starts after the trial has expired and lasts for one week. It allows a user who may have forgotten or
// otherwise let a trial temporarily lapse to continue to use the app for a short period, with the hope they will buy a
// subscription.
func (u *User) InTrialGracePeriod() bool {
	return !u.HasValidSubscription() &&
		time.Now().UTC().After(u.CreatedAt.Add(DefaultTrialPeriod)) &&
		time.Now().UTC().Before(u.CreatedAt.Add(DefaultTrialPeriod+7*24*time.Hour))
}

// HasValidSubscription returns a boolean indicating whether the user has a valid subscription. A valid subscription
// only indicates the user has a subscription of some type. It does not indicate the state of the subscription.
func (u *User) HasValidSubscription() bool {
	switch {
	case u.Subscription == nil:
		// No subscription data.
		return false
	case u.UserSubscriptionType != nil && *u.UserSubscriptionType != "":
		// Has subscription data. Validate subscription data.
		switch *u.UserSubscriptionType {
		case UserSubscriptionTypePaddle:
			subscription, err := u.Subscription.AsPaddleSubscription()
			if err != nil {
				return false
			}
			if err = validation.Validate.Struct(subscription); err != nil {
				return false
			}
			return true
		case UserSubscriptionTypeAndroid:
			subscription, err := u.Subscription.AsAndroidSubscription()
			if err != nil {
				return false
			}
			if err = validation.Validate.Struct(subscription); err != nil {
				return false
			}
			return true
		}
	default:
		// Invalid or incomplete subscription data.
		return false
	}
	return false
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
	// Multiply parsed MaxViewHistory by hours to get nanoseconds.
	s.MaxViewHistory = s.MaxViewHistory * time.Hour
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
	s.Email = validation.SanitizeString(s.Email)
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
