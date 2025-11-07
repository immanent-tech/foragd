// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/immanent-tech/foragd/validation"
)

const (
	// DefaultUserTheme is the default theme for the app.
	DefaultUserTheme = "silk"

	BasicAccountMaxHistory          = 7 * 24 * time.Hour // One week.
	BasicAccountUpdatesFrequency    = time.Hour
	StandardAccountMaxHistory       = 30 * 24 * time.Hour // One month.
	StandardAccountUpdatesFrequency = 5 * time.Minute
	PremiumAccountMaxHistory        = 365 * 24 * time.Hour // One year.
	PremiumAccountUpdatesFrequency  = time.Minute
)

var (
	ErrUserNotSubscribed    = errors.New("not subscribed")
	ErrUserAlreadyFavorited = errors.New("already a favorite")
)

// NewUser creates a new user from the external provider details.
func NewUser(externalID, email, provider string, level UserLevel) *User {
	ts := time.Now().UTC()
	user := &User{
		CreatedAt:      ts,
		UpdatedAt:      ts,
		ExternalUserId: externalID,
		Email:          email,
		Provider:       provider,
		UserID:         NewID(UserPFX),
		Settings:       *NewUserSettings(),
		Level:          level,
	}
	// Set account level based user settings.
	switch user.Level {
	case UserLevelBasic:
		user.Settings.MaxHistory = BasicAccountMaxHistory.String()
		user.Settings.UpdatesFrequency = BasicAccountUpdatesFrequency.String()
	case UserLevelStandard:
		user.Settings.MaxHistory = StandardAccountMaxHistory.String()
		user.Settings.UpdatesFrequency = StandardAccountUpdatesFrequency.String()
	case UserLevelCustom, UserLevelPremium:
		user.Settings.MaxHistory = PremiumAccountMaxHistory.String()
		user.Settings.UpdatesFrequency = PremiumAccountUpdatesFrequency.String()
	}
	return user
}

// Valid returns a boolean indicating whether the user data is valid. If not valid, it will also return a non-nil error
// that contains the validation issues.
func (u *User) Valid(_ context.Context) (bool, error) {
	err := validation.Validate.Struct(u)
	if err != nil {
		return false, fmt.Errorf("user data is invalid: %w", err)
	}
	return true, nil
}

// GetID returns the ID for the user.
func (u *User) GetID() UserID {
	return u.UserID
}

// GetAvatar retrieves the URL to the image to represent the user.
func (u *User) GetAvatar() string {
	return u.AvatarURL
}

// GetNickname retrieves the nickname of the user.
func (u *User) GetNickname() string {
	return u.Nickname
}

// GetEmail retrieves the email of the user.
func (u *User) GetEmail() string {
	return u.Email
}

// GetMaxHistory returns a timestamp in the past from which the user can view
// items.
func (u *User) GetMaxHistory() time.Time {
	return parseMaxHistory(u.GetSettings().MaxHistory)
}

// GetSettings returns the user's settings. If the user has no settings (i.e. new user), default settings will be
// returned.
func (u *User) GetSettings() *UserSettings {
	return &u.Settings
}

// GetSubscriptions retrieves a slice of the user subscriptions.
func (u *User) GetSubscriptions(options ...subscriptionFilterOption) []*Subscription {
	subscriptions := u.Subscriptions
	// Apply filtering options.
	for option := range slices.Values(options) {
		subscriptions = option(subscriptions)
	}
	return subscriptions
}

type subscriptionFilterOption func([]*Subscription) []*Subscription

// FilterByIDs option will filter the subscriptions by the given IDs.
func FilterByIDs(ids ...SubscriptionID) subscriptionFilterOption {
	return func(s []*Subscription) []*Subscription {
		if len(ids) == 0 {
			return s
		}
		return slices.Collect(
			FilterSlice(s, func(e *Subscription) bool {
				return slices.Contains(ids, e.GetID())
			}),
		)
	}
}

// SortByTitle sorts the subscriptions by their title.
func SortByTitle() subscriptionFilterOption {
	return func(s []*Subscription) []*Subscription {
		sort.Slice(s, func(i, j int) bool { return s[i].GetTitle() < s[j].GetTitle() })
		return s
	}
}

// GetSubscriptionByID returns the Subscription that matches the given ID or nil if none match.
func (u *User) GetSubscriptionByID(id SubscriptionID) *Subscription {
	if idx := slices.IndexFunc(u.GetSubscriptions(), func(e *Subscription) bool {
		return e.GetID() == id
	}); idx != -1 {
		return u.GetSubscriptions()[idx]
	}
	return nil
}

// GetFeedSubscriptions returns a new slice containing just the FeedSubscription subscriptions.
func (u *User) GetFeedSubscriptions() FeedSubscriptions {
	feedSubscriptions := make(FeedSubscriptions, 0, len(u.GetSubscriptions()))
	for anySubscription := range slices.Values(u.GetSubscriptions()) {
		if anySubscription.GetType() == SubscriptionTypeFeed {
			subscription, err := anySubscription.Data.AsFeedSubscription()
			if err != nil {
				continue
			}
			subscription.Metadata = anySubscription.Metadata
			subscription.Favorite = anySubscription.Favorite
			feedSubscriptions = append(feedSubscriptions, &subscription)
		}
	}
	return feedSubscriptions
}

// GetSearchSubscriptions returns a new slice containing just the SearchSubscription subscriptions.
func (u *User) GetSearchSubscriptions() SearchSubscriptions {
	searchSubscriptions := make(SearchSubscriptions, 0, len(u.GetSubscriptions()))
	for anySubscription := range slices.Values(u.GetSubscriptions()) {
		if anySubscription.GetType() == SubscriptionTypeSearch {
			subscription, err := anySubscription.Data.AsSearchSubscription()
			if err != nil {
				continue
			}
			subscription.Metadata = anySubscription.Metadata
			searchSubscriptions = append(searchSubscriptions, &subscription)
		}
	}
	return searchSubscriptions
}

// IsSubscribedToFeed returns a boolean indicating whether the user is subscribed to a feed with the given id.
func (u *User) IsSubscribedToFeed(id FeedID) bool {
	return u.GetFeedSubscriptions().GetByID(id) != nil
}

// UpdateFeedSubscription updates a FeedSubscription for the user.
func (u *User) UpdateFeedSubscription(update *FeedSubscription) error {
	// TODO: validation?
	subscription := u.GetSubscriptionByID(update.GetID())
	subscription.Metadata = update.Metadata
	subscription.Metadata.UpdatedAt = time.Now().UTC()
	err := subscription.Data.FromFeedSubscription(*update)
	if err != nil {
		return fmt.Errorf("could not update subscription: %w", err)
	}
	idx := slices.IndexFunc(u.Subscriptions, func(e *Subscription) bool {
		return e.GetID() == update.GetID()
	})
	if idx != -1 {
		u.Subscriptions[idx] = subscription
		return nil
	}
	return ErrUserNotSubscribed
}

// RemoveSubscriptions removes the user subscriptions with the matching id.
func (u *User) RemoveSubscriptions(ids ...SubscriptionID) {
	u.Subscriptions = slices.Collect(
		FilterSlice(u.Subscriptions, func(e *Subscription) bool {
			return !slices.Contains(ids, e.GetID())
		}),
	)
}

// MarkSubscriptions marks user subscriptions with the given ids with the given mark.
func (u *User) MarkSubscriptions(mark Mark, ids ...SubscriptionID) {
	for subscription := range slices.Values(u.GetSubscriptions(FilterByIDs(ids...))) {
		switch mark {
		case MarkRead:
			// Set marked at to now when marking read.
			subscription.Metadata.MarkedReadAt = time.Now().UTC()
		case MarkUnread:
			// Set marked at to max history when marking unread.
			subscription.Metadata.MarkedReadAt = u.GetMaxHistory()
		}
		// Reset subscription item states as well.
		if _, found := u.ItemStates[subscription.GetID()]; found {
			u.ItemStates[subscription.GetID()] = nil
		}
	}
}

// GetUnreadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as unread.
func (u *User) GetUnreadItems(id SubscriptionID) []ItemID {
	itemStates, found := u.ItemStates[id]
	if !found {
		return nil
	}
	ids := make([]ItemID, 0, len(itemStates))
	for id, state := range itemStates {
		if !state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (u *User) GetReadItems(id SubscriptionID) []ItemID {
	itemStates, found := u.ItemStates[id]
	if !found {
		return nil
	}
	ids := make([]ItemID, 0, len(itemStates))
	for id, state := range itemStates {
		if state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (u *User) GetItemState(subscriptionID SubscriptionID, itemID ItemID) *ArticleState {
	itemStates, found := u.ItemStates[subscriptionID]
	if found {
		// Retrieve any explicitly set state of the item.
		if state, found := itemStates[itemID]; found {
			return &state
		}
	}
	// If an item doesn't have an explicit state, its state should reflect the subscription state.
	return &ArticleState{
		Read:      false,
		UpdatedAt: u.GetSubscriptionByID(subscriptionID).Metadata.UpdatedAt,
	}
}

// SetItemState will set the state of the item to the given state.
func (u *User) SetItemState(subscriptionID SubscriptionID, itemID ItemID, state *ArticleState) {
	if u.ItemStates == nil {
		u.ItemStates = make(map[SubscriptionID]map[ItemID]ArticleState)
	}
	if u.ItemStates[subscriptionID] == nil {
		u.ItemStates[subscriptionID] = make(map[ItemID]ArticleState)
	}
	u.ItemStates[subscriptionID][itemID] = *state
}

// MarkItemsRead will mark the given items as read for the subscription.
func (u *User) MarkItemsRead(subscriptionID SubscriptionID, itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if !u.GetItemState(subscriptionID, itemID).Read {
			u.SetItemState(subscriptionID, itemID, &ArticleState{Read: true, UpdatedAt: time.Now().UTC()})
		}
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (u *User) MarkItemsUnread(subscriptionID SubscriptionID, itemIDs ...ItemID) {
	for itemID := range slices.Values(itemIDs) {
		if u.GetItemState(subscriptionID, itemID).Read {
			u.SetItemState(subscriptionID, itemID, &ArticleState{Read: false, UpdatedAt: time.Now().UTC()})
		}
	}
}

// MarkItems marks the given items in a user subscription the given mark.
func (u *User) MarkItems(mark Mark, subscriptionID SubscriptionID, itemIDs ...ItemID) {
	switch mark {
	case MarkRead:
		u.MarkItemsRead(subscriptionID, itemIDs...)
	case MarkUnread:
		u.MarkItemsUnread(subscriptionID, itemIDs...)
	}
	// u.Subscriptions[idx].UpdatedAt = time.Now().UTC()
}

// NewUserSettings returns a new instance of the default user settings.
func NewUserSettings() *UserSettings {
	return &UserSettings{
		Theme:                 DefaultUserTheme,
		ShowOnboarding:        true,
		ShowUnreadCounts:      true,
		MarkArticleReadOnView: true,
		MaxHistory:            BasicAccountMaxHistory.String(),
	}
}

// Valid returns a boolean indicating if the UserSettings contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *UserSettings) Valid() (bool, error) {
	err := validation.Validate.Struct(s)
	if err != nil {
		return false, fmt.Errorf("invalid user settings: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise UserSettings values.
func (s *UserSettings) Sanitise() error {
	return nil
}

// GetUserTheme returns the current user's theme or the default theme if no user theme is set.
func GetUserTheme(ctx context.Context) string {
	user, err := UserFromCtx(ctx)
	if err != nil {
		return DefaultUserTheme
	}
	if theme := user.GetSettings().Theme; theme != "" {
		return theme
	}
	return DefaultUserTheme
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *EditUserRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(s)
	if err != nil || !valid {
		return false, fmt.Errorf("request is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the user input for a SubscriptionCustomisation.
func (s *EditUserRequest) Sanitise() error {
	s.Nickname = validation.SanitizeString(s.Nickname)
	return nil
}

// Valid returns a boolean indicating whether the ChangePasswordRequest contains valid data.
func (r *ChangePasswordRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(r)
	if err != nil || !valid {
		return false, fmt.Errorf("request is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the user input for a ChangePasswordRequest.
func (r *ChangePasswordRequest) Sanitise() error {
	return nil
}
