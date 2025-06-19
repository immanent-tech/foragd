// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/joshuar/go-feed-me/components/validation"
	"github.com/joshuar/go-feed-me/models/feeds/types"
)

var ErrInvalidSubscriptionState = errors.New("invalid subscription state")

// GenerateSubscription creates a subscription from the given data sources: a feed, any user customisation of feed
// values, subscription state and an unread count. All data besides the feed is optional.
func GenerateSubscription(userID UserID, feed *Feed, customisation *SubscriptionCustomisation, state *SubscriptionState, unread int) (*Subscription, error) {
	subscription := &Subscription{
		UserID:        userID,
		Feed:          feed,
		Customisation: &ObjectCustomisation{},
	}
	// Create a new subscription object.
	if state == nil {
		return nil, fmt.Errorf("unable to generate subscription: %w", ErrInvalidSubscriptionState)
	}
	subscription.SubscriptionID = state.GetID()
	subscription.State = state.State
	// Add any user customisations.
	if customisation != nil {
		if customisation.Title != "" {
			subscription.Customisation.Title = customisation.Title
		}
		if customisation.Categories != nil {
			subscription.Customisation.Categories = customisation.Categories
		}
	}
	// Add unread count.
	subscription.UnreadCount = unread
	// Validate the subscription.
	if valid, err := subscription.Valid(); !valid {
		return nil, fmt.Errorf("subscription data is invalid: %w", err)
	}

	return subscription, nil
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *Subscription) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(s); err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

func (s *Subscription) String() string {
	if s.GetTitle() != "" {
		return fmt.Sprintf("%s (%s)", s.GetTitle(), s.Feed.GetSourceURL())
	}
	return s.Feed.GetSourceURL()
}

func (s *Subscription) GetID() SubscriptionID {
	return s.SubscriptionID
}

func (s *Subscription) GetFeedID() FeedID {
	return s.Feed.GetID()
}

func (s *Subscription) GetTitle() string {
	if s.Customisation.Title != "" {
		return s.Customisation.Title
	}
	return s.Feed.GetTitle()
}

func (s *Subscription) GetLink() string {
	return s.Feed.GetSourceURL()
}

func (s *Subscription) GetDescription() string {
	return s.Feed.GetDescription()
}

func (s *Subscription) GetCategories() []Category {
	if s.Customisation.Categories != nil {
		return slices.Compact(slices.Concat(s.Customisation.Categories, s.Feed.GetCategories()))
	}
	return s.Feed.GetCategories()
}

func (s *Subscription) GetAuthors() []string {
	return s.Feed.GetAuthors()
}

func (s *Subscription) GetUpdatedDate() time.Time {
	return s.Feed.GetUpdatedDate()
}

func (s *Subscription) GetImage() *types.Image {
	return s.Feed.GetImage()
}

func (s *Subscription) GetUnreadCount() int {
	return s.UnreadCount
}

func (s *Subscription) SetUnreadCount(count int) {
	s.UnreadCount = count
}

// IsUnread returns a boolean indicating whether the subscription is considered unread.
func (s *Subscription) IsUnread() bool {
	return s.UnreadCount > 0
}

type Subscriptions []*Subscription

// Valid returns a boolean indicating whether the SubscriptionRequest is valid,
// and any validation errors if applicable.
func (r *SubscriptionRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(r)
	if !valid || err != nil {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the input values of the SubscriptionRequest.
func (r *SubscriptionRequest) Sanitise() error {
	r.URL = validation.SanitizeString(r.URL)
	if r.Nickname != nil {
		sanitizedNickname := validation.SanitizeString(*r.Nickname)
		r.Nickname = &sanitizedNickname
	}
	categories := make([]Category, 0, len(r.Categories))
	for category := range slices.Values(r.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	r.Categories = categories
	return nil
}

func (r *SubscriptionRequest) String() string {
	if r.GetNickname() != "" {
		return fmt.Sprintf("%s (%s)", r.GetNickname(), r.GetURL())
	}
	return fmt.Sprintf("(%s)", r.GetURL())
}

func (r *SubscriptionRequest) GetURL() string {
	return strings.TrimSpace(r.URL)
}

func (r *SubscriptionRequest) GetNickname() string {
	if r.Nickname != nil {
		return *r.Nickname
	}
	return ""
}

func (r *SubscriptionRequest) GenerateCustomisation(id SubscriptionID, userID UserID, feedID FeedID) *SubscriptionCustomisation {
	if r.GetNickname() != "" || len(r.Categories) > 0 {
		return &SubscriptionCustomisation{
			UserID:         userID,
			FeedID:         feedID,
			SubscriptionID: id,
			Categories:     r.Categories,
			Title:          r.GetNickname(),
		}
	}
	return nil
}

type SubscriptionRequests []*SubscriptionRequest

// NewSubscriptionState creates a new subscription state with the given subscription and feed ids.
func NewSubscriptionState(feedID FeedID) *SubscriptionState {
	return &SubscriptionState{
		SubscriptionID: NewID(SubscriptionPFX),
		FeedID:         feedID,
		State:          NewObjectState(),
		ItemStates:     make(map[ItemID]ObjectState),
	}
}

func (s *SubscriptionState) GetID() SubscriptionID {
	return s.SubscriptionID
}

func (s *SubscriptionState) GetFeedID() FeedID {
	return s.FeedID
}

func (s *SubscriptionState) IsRead() bool {
	return s.State.IsRead()
}

// GetMarkedRead retrieves the timestamp when the user last marked the
// subscription feed as read.
func (s *SubscriptionState) GetMarkedRead() time.Time {
	return s.State.GetLastUpdate()
}

// MarkRead will mark the subscription as read. This involves setting the MarkedRead field to the given value and
// removing any individual unread/read items.
func (s *SubscriptionState) MarkRead(markedAt time.Time) {
	s.State.MarkRead(markedAt)
	s.ItemStates = nil
}

// GetUnreadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as unread.
func (s *SubscriptionState) GetUnreadItems() []ItemID {
	var ids []ItemID
	for id, state := range s.ItemStates {
		if !state.IsRead() {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (s *SubscriptionState) GetReadItems() []ItemID {
	var ids []ItemID
	for id, state := range s.ItemStates {
		if state.IsRead() {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *SubscriptionState) GetItemState(id ItemID) *ObjectState {
	if state, found := s.ItemStates[id]; found {
		return &state
	}
	return NewObjectState()
}

func (s *SubscriptionState) SetItemState(id ItemID, state *ObjectState) {
	if s.ItemStates == nil {
		s.ItemStates = make(map[ItemID]ObjectState)
	}
	s.ItemStates[id] = *state
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *SubscriptionState) MarkItemsRead(ids ...ItemID) {
	for id := range slices.Values(ids) {
		state := s.GetItemState(id)
		if state == nil {
			state = NewObjectState()
		}
		state.MarkRead(time.Now().UTC())
		s.SetItemState(id, state)
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (s *SubscriptionState) MarkItemsUnread(ids ...ItemID) {
	for id := range slices.Values(ids) {
		state := s.GetItemState(id)
		if state == nil {
			state = NewObjectState()
		}
		state.MarkUnread(time.Now().UTC())
		s.SetItemState(id, state)
	}
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *SubscriptionState) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(s); err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// SubscriptionStates is a map of subscription states by either subscription or feed id.
type SubscriptionStates[T comparable] map[T]*SubscriptionState

// GetIDsFromStates retrieves the subscription ids from the map of subscription states.
func GetIDsFromStates[T comparable](states SubscriptionStates[T]) []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(states))
	for _, state := range states {
		ids = append(ids, state.GetID())
	}
	return ids
}

func (s *SubscriptionCustomisation) GetID() SubscriptionID {
	return s.SubscriptionID
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *SubscriptionCustomisation) Valid() (bool, error) {
	if valid, err := validation.ValidateStruct(s); err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the user input for a SubscriptionCustomisation.
func (s *SubscriptionCustomisation) Sanitise() error {
	s.Title = validation.SanitizeString(s.Title)
	categories := make([]Category, 0, len(s.Title))
	for category := range slices.Values(s.Categories) {
		category = validation.SanitizeString(category)
		categories = append(categories, category)
	}
	s.Categories = categories
	return nil
}

// SubscriptionCustomisations is a slice of subscription customisation data.
type SubscriptionCustomisations []*SubscriptionCustomisation

// GetCustomisation will retrieve any customisation values for the subscription with the given id from the slice of
// subscription customisation data.
func (c SubscriptionCustomisations) GetCustomisation(id SubscriptionID) *SubscriptionCustomisation {
	if idx := slices.IndexFunc(c, func(c *SubscriptionCustomisation) bool {
		return c.SubscriptionID == id
	}); idx != -1 {
		return c[idx]
	}
	return nil
}
