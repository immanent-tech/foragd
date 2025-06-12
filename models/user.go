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

const DefaultSettingTheme = "dracula"

var (
	ErrAddUser               = errors.New("add subscription failed")
	ErrUpdateUser            = errors.New("update user failed")
	ErrUserAlreadyReadItem   = errors.New("user already read this item")
	ErrUserAlreadyUnreadItem = errors.New("user already unread this item")
	ErrNotSubscribed         = errors.New("user not subscribed to feed")
)

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
	if u.Settings != nil {
		return u.Settings
	}
	return NewUserSettings()
}

// MarkSubscriptions will mark either the given list of subscriptions, or all user subscriptions if none given, with the
// given mark.
func (u *User) MarkSubscriptions(mark Mark, subscriptionIDs ...SubscriptionID) {
	// Based on the requested state change, calculate the marked read timestamp
	// for the feed.
	// For read state, this will be the current time.
	// For unread state, this will be the max history of the user.
	var markedAt time.Time
	switch mark {
	case MarkRead:
		markedAt = time.Now().UTC()
	case MarkUnread:
		markedAt = u.GetMaxHistory()
	}

	switch {
	case len(subscriptionIDs) == 0:
		// Mark all subscriptions.
		for subscription := range slices.Values(u.Subscriptions) {
			subscription.MarkRead(markedAt)
		}
	default:
		// Mark the selected subscriptions.
		for subscription := range slices.Values(u.Subscriptions) {
			if slices.Contains(subscriptionIDs, subscription.GetID()) {
				subscription.MarkRead(markedAt)
			}
		}
	}
}

// AddSubscriptions adds the given Subscriptions to the User.
func (u *User) AddSubscriptions(details ...*SubscriptionDetails) {
	for subscription := range slices.Values(details) {
		u.Subscriptions = append(u.Subscriptions, *subscription)
	}
}

func (u *User) GetAllSubscriptions() []*SubscriptionDetails {
	subscriptions := make([]*SubscriptionDetails, 0, len(u.Subscriptions))
	for details := range slices.Values(u.Subscriptions) {
		subscriptions = append(subscriptions, &details)
	}
	return subscriptions
}

func (u *User) GetSubscriptions(ids ...SubscriptionID) []*SubscriptionDetails {
	subscriptions := make([]*SubscriptionDetails, 0, len(u.Subscriptions))
	for details := range FilterSlice(u.Subscriptions, func(details SubscriptionDetails) bool {
		return slices.Contains(ids, details.GetID())
	}) {
		subscriptions = append(subscriptions, &details)
	}
	return subscriptions
}

func (u *User) GetSubscriptionByFeedID(id FeedID) *SubscriptionDetails {
	idx := slices.IndexFunc(u.Subscriptions, func(details SubscriptionDetails) bool {
		return details.GetFeedID() == id
	})
	if idx != -1 {
		return &u.Subscriptions[idx]
	}
	return nil
}

// func (u *User) FilterSubscriptionsByID(ids ...SubscriptionID) []*SubscriptionDetails {
// 	subscriptions := make([]*SubscriptionDetails, 0, len(ids))
// 	for subscription := range FilterSlice(u.Subscriptions, func(details SubscriptionDetails) bool {
// 		return slices.Contains(ids, details.GetSubscriptionID())
// 	}) {
// 		subscriptions = append(subscriptions, &subscription)
// 	}
// 	return subscriptions
// }

// RemoveSubscriptions removes the given Subscriptions from the User.
func (u *User) RemoveSubscriptions(subscriptionIDs ...SubscriptionID) {
	for id := range slices.Values(subscriptionIDs) {
		u.Subscriptions = slices.DeleteFunc(u.Subscriptions, func(s SubscriptionDetails) bool {
			return id == s.GetID()
		})
	}
}

// EditSubscription will apply the given user customisation to the subscription with the given ID.
func (u *User) EditSubscription(subscriptionID SubscriptionID, edits *SubscriptionCustomisation) {
	idx := slices.IndexFunc(u.Subscriptions, func(v SubscriptionDetails) bool { return v.GetID() == subscriptionID })
	if idx != -1 {
		// Update categories.
		u.Subscriptions[idx].UserCategories = edits.UserCategories
		// Update nickname.
		u.Subscriptions[idx].UserNickname = edits.UserNickname
	}
}

// IsSubscribed returns a boolean indicating whether the user is subscribed to the feed with the given ID.
func (u *User) IsSubscribed(id FeedID) bool {
	return slices.ContainsFunc(u.Subscriptions, func(details SubscriptionDetails) bool {
		return details.GetFeedID() == id
	})
}

// MarkItems will mark all items for the given feed with the given mark for the user.
func (u *User) MarkItems(mark Mark, feedID FeedID, itemIDs ...ItemID) {
	idx := slices.IndexFunc(u.Subscriptions, func(v SubscriptionDetails) bool { return v.GetFeedID() == feedID })
	if idx != -1 {
		switch mark {
		case MarkRead:
			u.Subscriptions[idx].MarkItemsRead(itemIDs...)
		case MarkUnread:
			u.Subscriptions[idx].MarkItemsUnread(itemIDs...)
		}
	}
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
		Theme: DefaultSettingTheme,
	}
}
