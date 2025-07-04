// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/components/validation"
)

const DefaultUserTheme = "dracula"

var (
	ErrAddUser               = errors.New("add subscription failed")
	ErrUpdateUser            = errors.New("update user failed")
	ErrUserAlreadyReadItem   = errors.New("user already read this item")
	ErrUserAlreadyUnreadItem = errors.New("user already unread this item")
	ErrNotSubscribed         = errors.New("user not subscribed to feed")
)

// Valid returns a boolean indicating whether the user data is valid. If not valid, it will also return a non-nil error
// that contains the validation issues.
func (u *User) Valid(_ context.Context) (bool, error) {
	return validation.ValidateStruct(u)
}

// GetID returns the ID for the user.
func (u *User) GetID() UserID {
	return u.UserID
}

// GetMaxHistory returns a timestamp in the past from which the user can view
// items.
func (u *User) GetMaxHistory() time.Time {
	return parseMaxHistory(u.MaxHistory)
}

// GetSettings returns the user's settings. If the user has no settings (i.e. new user), default settings will be
// returned.
func (u *User) GetSettings() *UserSettings {
	if u.Settings == nil {
		return NewUserSettings()
	}
	return u.Settings
}

// GetSubscriptionState returns the subscription state matching the given id.
func (u *User) GetSubscriptionState(id SubscriptionID) *SubscriptionState {
	for state := range slices.Values(u.Subscriptions) {
		if state.GetID() == id {
			return &state
		}
	}
	return nil
}

// GetAllSubscriptionStates returns a map of subscription states by subscription id.
func (u *User) GetAllSubscriptionStates() map[SubscriptionID]*SubscriptionState {
	return SliceToMap(u.Subscriptions, func(s SubscriptionState) (SubscriptionID, *SubscriptionState) {
		return s.SubscriptionID, &s
	})
}

// FilterSubscriptionStatesByID returns a map of subscription states by subscription id, filtered by the given
// subscription ids.
func (u *User) FilterSubscriptionStatesByID(ids ...SubscriptionID) map[SubscriptionID]*SubscriptionState {
	return SliceToMap(slices.Collect(FilterSlice(u.Subscriptions, func(s SubscriptionState) bool {
		return slices.Contains(ids, s.GetID())
	})), func(s SubscriptionState) (SubscriptionID, *SubscriptionState) {
		return s.SubscriptionID, &s
	})
}

// GetAllSubscriptionStatesByFeed returns a map of subscription states by feed id.
func (u *User) GetAllSubscriptionStatesByFeed() map[FeedID]*SubscriptionState {
	return SliceToMap(u.Subscriptions, func(s SubscriptionState) (FeedID, *SubscriptionState) {
		return s.GetFeedID(), &s
	})
}

// FilterSubscriptionStatesByFeedID returns a map of subscription states by feed id, filtered by the given
// feed ids.
func (u *User) FilterSubscriptionStatesByFeed(ids ...FeedID) map[FeedID]*SubscriptionState {
	return SliceToMap(slices.Collect(FilterSlice(u.Subscriptions, func(s SubscriptionState) bool {
		return slices.Contains(ids, s.GetFeedID())
	})), func(s SubscriptionState) (FeedID, *SubscriptionState) {
		return s.GetFeedID(), &s
	})
}

// IsSubscribedToFeed returns a boolean indicating whether the user has a subscription to the feed with the given feed id.
func (u *User) IsSubscribedToFeed(feedID FeedID) bool {
	for state := range slices.Values(u.Subscriptions) {
		if state.GetFeedID() == feedID {
			return true
		}
	}
	return false
}

// HasSubscription returns a boolean indicating whether the user has a subscription with the given id.
func (u *User) HasSubscription(id SubscriptionID) bool {
	for state := range slices.Values(u.Subscriptions) {
		if state.GetID() == id {
			return true
		}
	}
	return false
}

// Valid will check to ensure the UserSignupRequest contains valid data.
func (u *UserSignupRequest) Valid() (bool, error) {
	_, problems := validation.ValidateStruct(u)
	if problems != nil {
		return false, fmt.Errorf("user is invalid: %w", problems)
	}

	return true, nil
}

// Sanitise will sanitise the UserSignupRequest.
func (u *UserSignupRequest) Sanitise() error {
	u.Email = validation.SanitizeString(u.Email)
	u.Nickname = validation.SanitizeString(u.Nickname)
	return nil
}

func NewUserSignup() *UserSignupRequest {
	return &UserSignupRequest{}
}

// NewUserSettings returns a new instance of the default user settings.
func NewUserSettings() *UserSettings {
	return &UserSettings{
		Theme: DefaultUserTheme,
	}
}

func GetUserTheme(ctx context.Context) string {
	if user, found := UserFromCtx(ctx); found {
		if theme := user.GetSettings().Theme; theme != "" {
			return theme
		}
	}
	return DefaultUserTheme
}
