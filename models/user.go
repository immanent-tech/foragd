// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/components/validation"
)

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

// GetMarkedRead retrieves the datetime when the user last marked the given Feed
// as read. If the Feed is unread, it will return the user's max history limit.
func (u *User) GetMarkedRead(id FeedID) time.Time {
	idx := slices.IndexFunc(u.Subscriptions, func(v *Subscription) bool {
		return v.GetFeedID() == id
	})
	if idx != -1 {
		if u.Subscriptions[idx].GetMarkedRead().IsZero() {
			return u.GetMaxHistory()
		}
		return u.Subscriptions[idx].GetMarkedRead()
	}
	return u.GetMaxHistory()
}

// GetSubscriptions retrieves all Subscriptions for the user.
func (u *User) GetSubscriptions() Subscriptions {
	return u.Subscriptions
}

// GetSubscriptionFeedIDs gets all FeedIDs for all Subscriptions for the user.
func (u *User) GetSubscriptionFeedIDs() []FeedID {
	return u.GetSubscriptions().GetFeedIDs()
}

// GetSubscriptionCategories gets all Categories for all Subscriptions for the user.
func (u *User) GetSubscriptionCategories() []Category {
	return u.GetSubscriptions().GetCategories()
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
func (u *User) AddSubscriptions(subscriptions Subscriptions) {
	for subscription := range slices.Values(subscriptions) {
		u.Subscriptions = append(u.Subscriptions, subscription)
	}
}

// RemoveSubscriptions removes the given Subscriptions from the User.
func (u *User) RemoveSubscriptions(subscriptionIDs ...SubscriptionID) {
	for id := range slices.Values(subscriptionIDs) {
		u.Subscriptions = slices.DeleteFunc(u.Subscriptions, func(s *Subscription) bool {
			return id == s.GetID()
		})
	}
}

// EditSubscription will apply the given user customisation to the subscription with the given ID.
func (u *User) EditSubscription(subscriptionID SubscriptionID, edits *SubscriptionCustomisation) {
	idx := slices.IndexFunc(u.Subscriptions, func(v *Subscription) bool { return v.GetID() == subscriptionID })
	if idx != -1 {
		// Update categories.
		u.Subscriptions[idx].UserCategories = edits.UserCategories
		// Update nickname.
		u.Subscriptions[idx].UserNickname = edits.UserNickname
	}
}

// IsSubscribed returns a boolean indicating whether the user is subscribed to the feed with the given ID.
func (u *User) IsSubscribed(feedID FeedID) bool {
	subscriptions := u.GetSubscriptions().FilterByFeedID(feedID)
	return len(subscriptions) > 0
}

// MarkItems will mark all items for the given feed with the given mark for the user.
func (u *User) MarkItems(mark Mark, feedID FeedID, itemIDs ...ItemID) {
	idx := slices.IndexFunc(u.Subscriptions, func(v *Subscription) bool { return v.GetFeedID() == feedID })
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
	valid, problems := validation.ValidateStruct(u)
	if problems != nil {
		u.Msg = NewMessage(
			"User details are invalid.",
			MessageStatusWarning,
			WithDetails(problems.Error()),
			WithError(problems),
		)
	}

	return valid, problems
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
