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
	ErrUserAlreadySubscribed = errors.New("user already subscribed")
	ErrUserAlreadyReadItem   = errors.New("user already read this item")
	ErrUserAlreadyUnreadItem = errors.New("user already unread this item")
	ErrNotSubscribed         = errors.New("user not subscribed to feed")
)

func (u *User) Valid(_ context.Context) (bool, validation.Problems) {
	return validation.ValidateStruct(u)
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
func (u *User) HasReadItem(feedID FeedID, itemID ItemID) bool {
	if u.FeedItemStates == nil {
		return false
	}

	items, foundFeed := u.FeedItemStates[feedID]
	if !foundFeed {
		return false
	}

	itemState, foundItem := items[itemID]
	if !foundItem {
		return false
	}

	return itemState.State == Read
}

// HasUnreadItem checks whether the item with the given ItemID has been marked unread by
// the user.
func (u *User) HasUnreadItem(feedID FeedID, itemID ItemID) bool {
	if u.FeedItemStates == nil {
		return false
	}

	items, foundFeed := u.FeedItemStates[feedID]
	if !foundFeed {
		return false
	}

	itemState, foundItem := items[itemID]
	if !foundItem {
		return false
	}

	return itemState.State == Unread
}

// GetItemState retrieves the user's state for the given item. If the item
// doesn't have a state, it returns nil.
func (u *User) GetItemState(feedID FeedID, itemID ItemID) *ItemState {
	if u.FeedItemStates == nil {
		return nil
	}

	items, foundFeed := u.FeedItemStates[feedID]
	if !foundFeed {
		return nil
	}

	itemState, foundItem := items[itemID]
	if !foundItem {
		return nil
	}

	return &itemState
}

// MarkItem marks an item with the given state for the user.
func (u *User) MarkItem(feedID FeedID, itemID ItemID, state State) error {
	if state == Read && u.HasReadItem(feedID, itemID) {
		return ErrUserAlreadyReadItem
	}

	if state == Unread && u.HasUnreadItem(feedID, itemID) {
		return ErrUserAlreadyUnreadItem
	}

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

// SubscriptionHasCategory returns whether the subscription with the given
// FeedID contains the given Category.
func (u *User) SubscriptionHasCategory(id FeedID, category Category) bool {
	return slices.Contains(u.Subscriptions[id].Categories, category)
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

func (u *User) GetCategoryCounts() []CategoryCount {
	counts := make(map[Category]int64)

	for _, subscription := range u.Subscriptions {
		for _, category := range subscription.Categories {
			// if _, found := counts[category]; !found {
			counts[category]++
			// }
		}
	}

	var categoryCounts []CategoryCount

	for category, count := range counts {
		categoryCounts = append(categoryCounts, CategoryCount{Name: category, Count: count})
	}

	return categoryCounts
}

// FilterSubscribedFeeds returns the user's subscribed feeds filtered by the
// given feed IDs.
func (u *User) FilterSubscribedFeeds(filters APIFilters) []FeedID {
	// If there are no relevant filters, return all subscribed Feed IDs.
	if len(filters.FeedIDs) == 0 && len(filters.Categories) == 0 {
		return u.GetSubscribedFeedIDs()
	}

	var filtered []FeedID

	switch {
	// Case 1: FeedID filters specified, no Category filters specified.
	case len(filters.FeedIDs) > 0 && len(filters.Categories) == 0:
		for _, id := range filters.FeedIDs {
			if u.IsSubscribed(id) {
				filtered = append(filtered, id)
			}
		}

		return filtered
	// Case 2: No FeedID filters specified, Category filters specified.
	case len(filters.FeedIDs) == 0 && len(filters.Categories) > 0:
		for id, details := range u.Subscriptions {
			for _, category := range details.Categories {
				if slices.Contains(filters.Categories, category) {
					filtered = append(filtered, id)
				}
			}
		}

		return filtered
	// Case 3: Both FeedID and Category filters specified
	default:
		for _, id := range filters.FeedIDs {
			if u.IsSubscribed(id) {
				for _, category := range filters.Categories {
					if u.SubscriptionHasCategory(id, category) {
						filtered = append(filtered, id)
					}
				}
			}
		}

		return filtered
	}
}

// AddSubscription adds a new subscription to the user object.
func (u *User) AddSubscription(ctx context.Context, api UserManagementAPI, feedID FeedID, details *APISubscriptionRequest) error {
	if u.IsSubscribed(feedID) {
		return ErrUserAlreadySubscribed
	}

	if u.Subscriptions == nil {
		u.Subscriptions = make(map[string]SubscriptionState)
	}

	u.Subscriptions[feedID] = NewSubscriptionState(details)

	partialUpdate := map[string]any{
		"subscriptions": u.Subscriptions,
	}

	if err := api.UpdateUser(ctx, u.ID, partialUpdate); err != nil {
		return errors.Join(ErrUpdateUser, err)
	}

	return nil
}

// GetMaxHistory returns a timestamp in the past after which the user can view
// items.
func (u *User) GetMaxHistory() time.Time {
	if u.MaxHistory == nil {
		return time.Now().Add(-defaultUserHistory)
	}

	dur, err := time.ParseDuration(*u.MaxHistory)
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
