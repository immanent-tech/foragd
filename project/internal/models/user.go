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
)

type UserPreferences map[string]any

func NewUserPreferences() UserPreferences {
	return map[string]any{
		"theme": "light",
	}
}

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

func (u *User) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(u)
}

func (u *User) IsSubscribed(id FeedID) bool {
	_, found := u.Subscriptions[id]
	return found
}

func (u *User) GetSubscribedFeedIDs() []FeedID {
	feedIDs := make([]FeedID, len(u.Subscriptions))
	idx := 0

	for feedID := range u.Subscriptions {
		feedIDs[idx] = feedID
		idx++
	}

	return feedIDs
}

func (u *User) AddSubscription(feedID FeedID, name string, categories []Category) error {
	if u.IsSubscribed(feedID) {
		return ErrUserAlreadySubscribed
	}

	if u.Subscriptions == nil {
		u.Subscriptions = make(map[string]Subscription)
	}

	u.Subscriptions[feedID] = Subscription{Name: name, Categories: categories, CreatedAt: time.Now()}

	return nil
}
