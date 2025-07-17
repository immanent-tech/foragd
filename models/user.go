// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/validation"
)

const DefaultUserTheme = "dracula"

var (
	ErrAddUser               = errors.New("add subscription failed")
	ErrUpdateUser            = errors.New("update user failed")
	ErrUserAlreadyReadItem   = errors.New("user already read this item")
	ErrUserAlreadyUnreadItem = errors.New("user already unread this item")
	ErrNotSubscribed         = errors.New("user not subscribed to feed")
)

// NewUser creates a new user from the external provider details.
func NewUser(externalID, provider string) *User {
	ts := time.Now().UTC()
	return &User{
		CreatedAt:      ts,
		UpdatedAt:      ts,
		MaxHistory:     DefaultMaxHistory.String(),
		ExternalUserId: externalID,
		Provider:       provider,
		UserID:         NewID(UserPFX),
	}
}

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
	return &u.Settings
}

func (u *User) AddSubscription(subscriptionID SubscriptionID, feedID FeedID) {
	u.Subscriptions = append(u.Subscriptions, SubscriptionFeedRelation{SubscriptionID: subscriptionID, FeedID: feedID})
}

func (u *User) GetSubscriptionsByID(ids ...SubscriptionID) map[SubscriptionID]FeedID {
	s := SliceToMap(u.Subscriptions, func(s SubscriptionFeedRelation) (SubscriptionID, FeedID) {
		return s.SubscriptionID, s.FeedID
	})
	if len(ids) == 0 {
		return s
	}
	return maps.Collect(FilterMap(s, func(subscriptionID SubscriptionID, _ FeedID) bool {
		return slices.Contains(ids, subscriptionID)
	}))
}

func (u *User) GetSubscriptionsByFeedID(ids ...FeedID) map[FeedID]SubscriptionID {
	s := SliceToMap(u.Subscriptions, func(s SubscriptionFeedRelation) (FeedID, SubscriptionID) {
		return s.FeedID, s.SubscriptionID
	})
	if len(ids) == 0 {
		return s
	}
	return maps.Collect(FilterMap(s, func(feedID FeedID, _ SubscriptionID) bool {
		return slices.Contains(ids, feedID)
	}))
}

// IsSubscribedToFeed returns a boolean indicating whether the user has a subscription to the feed with the given feed id.
func (u *User) IsSubscribedToFeed(feedID FeedID) bool {
	return slices.ContainsFunc(u.Subscriptions, func(s SubscriptionFeedRelation) bool {
		return s.FeedID == feedID
	})
}

// HasSubscription returns a boolean indicating whether the user has a subscription with the given id.
func (u *User) HasSubscription(id SubscriptionID) bool {
	return slices.ContainsFunc(u.Subscriptions, func(s SubscriptionFeedRelation) bool {
		return s.SubscriptionID == id
	})
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

// GetUserTheme returns the current user's theme or the default theme if no user theme is set.
func GetUserTheme(ctx context.Context) string {
	if user, found := UserFromCtx(ctx); found {
		if theme := user.GetSettings().Theme; theme != "" {
			return theme
		}
	}
	return DefaultUserTheme
}
