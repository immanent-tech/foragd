// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"slices"
	"time"
)

var (
	ErrAddUser               = errors.New("add subscription failed")
	ErrUserAlreadySubscribed = errors.New("user already subscribed")
	ErrUserAlreadyReadItem   = errors.New("user already read this item")
)

func (u *User) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(u)
}

// GetReadItemIDs fetches the ItemIDs for all read items for a user.
func (u *User) GetReadItemIDs(feedIDs ...FeedID) []ItemID {
	var readItemsIDs []ItemID

	for feedID, items := range u.ReadItems {
		if len(feedIDs) > 0 {
			if !slices.Contains(feedIDs, feedID) {
				continue
			}
		}

		for _, item := range items {
			readItemsIDs = append(readItemsIDs, item.ItemID)
		}
	}

	return readItemsIDs
}

// HasReadItem checks whether the item with the given ItemID has been read by
// the user.
func (u *User) HasReadItem(id ItemID) bool {
	_, found := u.ReadItems[id]
	return found
}

// MarkItemRead marks an item read in the user object.
func (u *User) MarkItemRead(item APIReadItem) error {
	if u.HasReadItem(item.ItemID) {
		return ErrUserAlreadyReadItem
	}

	if u.ReadItems == nil {
		u.ReadItems = make(map[string][]ReadItem)
	}

	u.ReadItems[item.FeedID] = append(u.ReadItems[item.FeedID], ReadItem{ItemID: item.ItemID, CreatedAt: time.Now().UTC()})

	return nil
}

// IsSubscribed checks if the user is subscribed to the feed with the given
// FeedID.
func (u *User) IsSubscribed(id FeedID) bool {
	_, found := u.Subscriptions[id]
	return found
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
func (u *User) AddSubscription(feedID FeedID, name string, categories []Category) error {
	if u.IsSubscribed(feedID) {
		return ErrUserAlreadySubscribed
	}

	if u.Subscriptions == nil {
		u.Subscriptions = make(map[string]Subscription)
	}

	createdAt := time.Now().UTC()

	u.Subscriptions[feedID] = Subscription{Name: name, Categories: categories, CreatedAt: &createdAt}

	return nil
}
