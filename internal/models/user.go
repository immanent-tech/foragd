// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/internal/validation"
)

const (
	defaultUserHistory = 30 * 24 * time.Hour
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
	if u.MaxHistory == "" {
		return time.Now().Add(-defaultUserHistory)
	}

	dur, err := time.ParseDuration(u.MaxHistory)
	if err != nil {
		return time.Now().Add(-defaultUserHistory)
	}

	return time.Now().Add(-dur)
}

// GetMarkedRead retrieves the datetime when the user last marked the given Feed
// as read. If the Feed is unread, it will return the user's max history limit.
func (u *User) GetMarkedRead(id FeedID) time.Time {
	idx := slices.IndexFunc(u.Subscriptions, func(v Subscription) bool {
		return v.GetFeedID() == id
	})
	if idx != -1 {
		if u.Subscriptions[idx].State.MarkedRead.IsZero() {
			return u.GetMaxHistory()
		}
		return u.Subscriptions[idx].State.MarkedRead
	}
	return u.GetMaxHistory()
}

// GetSubscriptions retrieves all Subscriptions for the user.
func (u *User) GetSubscriptions() Subscriptions {
	subscriptions := make(Subscriptions, 0, len(u.Subscriptions))
	for subscription := range slices.Values(u.Subscriptions) {
		subscriptions = append(subscriptions, &subscription)
	}
	return subscriptions
}

// GetSubscriptionFeedIDs gets all FeedIDs for all Subscriptions for the user.
func (u *User) GetSubscriptionFeedIDs() []FeedID {
	return u.GetSubscriptions().GetFeedIDs()
}

// GetSubscriptionCategories gets all Categories for all Subscriptions for the user.
func (u *User) GetSubscriptionCategories() []Category {
	return u.GetSubscriptions().GetCategories()
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (u *User) MarkSubscriptions(mark Mark, feedIDs ...FeedID) {
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

	// Mark the subscriptions.
	for subscription := range slices.Values(u.Subscriptions) {
		if slices.Contains(feedIDs, subscription.GetFeedID()) {
			updated := time.Now().UTC()
			subscription.State.MarkedRead = markedAt
			subscription.State.UnreadItems = nil
			subscription.State.ReadItems = nil
			subscription.UpdatedAt = &updated
		}
	}
}

// AddSubscriptions adds the given Subscriptions to the User.
func (u *User) AddSubscriptions(subscriptions Subscriptions) {
	for subscription := range slices.Values(subscriptions) {
		u.Subscriptions = append(u.Subscriptions, *subscription)
	}
}

func (u *User) IsSubscribed(feedID FeedID) bool {
	subscriptions := u.GetSubscriptions().FilterByFeedID(feedID)
	return len(subscriptions) > 0
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (u *User) MarkItems(mark Mark, itemIDs ...ItemID) {
	return
}

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

func NewUserSignup() *UserSignupRequest {
	return &UserSignupRequest{}
}
