// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"iter"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/oapi-codegen/nullable"
)

const (
	defaultUserHistory = 30 * 24 * time.Hour
)

var (
	ErrAddUser               = errors.New("add subscription failed")
	ErrUserAlreadySubscribed = errors.New("user already subscribed")
	ErrUserAlreadyReadItem   = errors.New("user already read this item")
	ErrNotSubscribed         = errors.New("user not subscribed to feed")
)

func (u *User) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(u)
}

// GetReadItemIDs fetches the ItemIDs for the given feeds with the given state.
func (u *User) GetItemIDs(state State, feedIDs ...FeedID) []ItemID {
	if len(feedIDs) == 0 {
		return nil
	}

	if u.ItemStates == nil {
		return nil
	}

	var itemIDs []ItemID

	for _, items := range u.ItemStates {
		readItems := slices.Collect(filterSlice(items, func(v ItemState) bool {
			if v.State.IsSpecified() {
				s, err := v.State.Get()
				if err != nil {
					return false
				}

				return s == state
			}

			return false
		}))

		for _, item := range readItems {
			itemIDs = append(itemIDs, item.ItemID)
		}
	}

	return itemIDs
}

// HasReadItem checks whether the item with the given ItemID has been read by
// the user.
func (u *User) HasReadItem(id ItemID) bool {
	for v := range maps.Values(u.ItemStates) {
		if slices.ContainsFunc(v, func(r ItemState) bool {
			return r.ItemID == id
		}) {
			return true
		}
	}

	return false
}

// MarkItemRead marks an item read in the user object.
func (u *User) MarkItem(feedID FeedID, itemID ItemID, state State) error {
	if u.HasReadItem(itemID) {
		return ErrUserAlreadyReadItem
	}

	if u.ItemStates == nil {
		u.ItemStates = make(map[string][]ItemState)
	}

	u.ItemStates[feedID] = append(u.ItemStates[feedID], NewItemState(itemID, state))

	return nil
}

// IsSubscribed checks if the user is subscribed to the feed with the given
// FeedID.
func (u *User) IsSubscribed(id FeedID) bool {
	_, found := u.Subscriptions[id]
	return found
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
	delete(u.ItemStates, feedID)

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

// AddSubscription adds a new subscription to the user object.
func (u *User) AddSubscription(feedID FeedID, name string, categories nullable.Nullable[Categories]) error {
	if u.IsSubscribed(feedID) {
		return ErrUserAlreadySubscribed
	}

	if u.Subscriptions == nil {
		u.Subscriptions = make(map[string]Subscription)
	}

	createdAt := time.Now().UTC()

	u.Subscriptions[feedID] = Subscription{Name: &name, Categories: categories, CreatedAt: &createdAt}

	return nil
}

// GetMaxHistory returns a timestamp in the past after which the user can view
// items.
func (u *User) GetMaxHistory() time.Time {
	dur, err := time.ParseDuration(u.MaxHistory)
	if err != nil {
		slog.Warn("Could not parse user max history. Using default.",
			slog.String("max_history", u.MaxHistory),
			slog.Any("error", err))

		return time.Now().Add(-defaultUserHistory)
	}

	return time.Now().Add(-dur)
}

func filterSlice[S any](s []S, fn func(S) bool) iter.Seq[S] {
	return func(yield func(s S) bool) {
		for _, v := range s {
			if fn(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}
