// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"maps"
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

func (u *User) Valid(_ context.Context) (bool, validation.ValidationErrors) {
	return validation.ValidateStruct(u)
}

// GetID returns the ID for the user.
func (u *User) GetID() UserID {
	return u.ID
}

// GetItemIDsWithState retrieves ItemIDs for the items the user has explicitly
// marked with the given state.
func (u *User) GetItemIDsWithState(state State, feedIDs ...FeedID) []ItemID {
	if len(feedIDs) == 0 {
		return nil
	}

	if u.FeedItemStates == nil {
		return nil
	}

	var itemIDs []ItemID

	for feedID, items := range u.FeedItemStates {
		if !slices.Contains(feedIDs, feedID) {
			continue
		}

		for itemID := range maps.Keys(items) {
			if items[itemID].State == state {
				itemIDs = append(itemIDs, itemID)
			}
		}
	}

	return itemIDs
}

// HasReadItem checks whether the item with the given ItemID has been marked read by
// the user.
func (u *User) HasReadItem(item *APIItem) bool {
	if itemState := u.GetItemState(item); itemState != nil {
		return itemState.State == Read
	}

	return false
}

// HasUnreadItem checks whether the item with the given ItemID has been marked unread by
// the user.
func (u *User) HasUnreadItem(item *APIItem) bool {
	if itemState := u.GetItemState(item); itemState != nil {
		return itemState.State == Unread
	}

	return false
}

// GetItemState retrieves the user's state for the given item. If the item
// doesn't have a state, it returns nil.
func (u *User) GetItemState(item *APIItem) *ItemState {
	// If the user has explicitly marked the item, return that state.
	if u.FeedItemStates != nil {
		if itemState, found := u.FeedItemStates[item.GetFeedID()][item.GetID()]; found {
			return &itemState
		}
	}
	// Else, if the item was published after the last time the feed was marked
	// read, return unread state.
	if item.GetTimestamp().After(u.GetFeedLastRead(item.GetFeedID())) {
		return &ItemState{State: Unread, UpdatedAt: item.GetTimestamp()}
	}
	// Else, return read state.
	return &ItemState{State: Read, UpdatedAt: u.GetFeedLastRead(item.GetFeedID())}
}

// MarkItem marks an item with the given state for the user.
func (u *User) MarkItem(feedID FeedID, itemID ItemID, state State) error {
	if u.FeedItemStates[feedID] == nil {
		u.FeedItemStates[feedID] = make(map[string]ItemState)
	}

	u.FeedItemStates[feedID][itemID] = newItemState(state)

	return nil
}

// MarkFeedRead marks a feed as read for the user.
func (u *User) MarkFeedRead(feedID FeedID, timestamp time.Time) error {
	if !u.IsSubscribed(feedID) {
		return ErrNotSubscribed
	}

	// Update the MarkedRead value for the feed subscription.
	subscription := u.Subscriptions[feedID]
	subscription.MarkedRead = &timestamp
	u.Subscriptions[feedID] = subscription
	// Delete any ReadItems records for this feed.
	delete(u.FeedItemStates, feedID)

	return nil
}

// GetFeedLastRead returns the timestamp when the user last marked the feed as
// read. If it does not exist, the user's max history timestamp is used.
func (u *User) GetFeedLastRead(feedID FeedID) time.Time {
	if sub, found := u.Subscriptions[feedID]; found {
		if sub.MarkedRead != nil {
			return *sub.MarkedRead
		}
	}

	return u.GetMaxHistory()
}

// IsSubscribed checks if the user is subscribed to the feed with the given
// FeedID.
func (u *User) IsSubscribed(id FeedID) bool {
	_, found := u.Subscriptions[id]
	return found
}

// GetSubscriptionName returns the user's nickname for the Feed with the given
// ID. If there is no custom name, an empty string is returned.
func (u *User) GetSubscriptionName(id FeedID) string {
	if subscription, found := u.Subscriptions[id]; found {
		return subscription.Name
	}

	return ""
}

// SubscriptionHasCategory returns whether the subscription with the given
// FeedID contains the given Category.
func (u *User) SubscriptionHasCategory(id FeedID, category Category) bool {
	return slices.Contains(u.Subscriptions[id].Categories, category)
}

// GetSubscriptionCategories returns the user's custom categories for the Feed with the given
// ID. If there are no custom categories, nil is returned.
func (u *User) GetSubscriptionCategories(id FeedID) Categories {
	if subscription, found := u.Subscriptions[id]; found {
		return subscription.Categories
	}

	return nil
}

// GetSubscribedFeedIDs fetches the FeedIDs for all the user's subscriptions.
func (u *User) GetSubscribedFeedIDs() []FeedID {
	feedIDs := make([]FeedID, len(u.Subscriptions))
	idx := 0

	for feedID := range u.Subscriptions {
		feedIDs[idx] = feedID
		idx++
	}

	return feedIDs
}

// GetMaxHistory returns a timestamp in the past after which the user can view
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

func newItemState(state State) ItemState {
	return ItemState{
		State:     state,
		UpdatedAt: time.Now().UTC(),
	}
}
