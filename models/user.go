// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/immanent-tech/foragd/validation"
)

const (
	// DefaultUserTheme is the default theme for the app.
	DefaultUserTheme = "forest"
	// DefaultMaxHistory for users/objects is 30 days.
	DefaultMaxHistory = 30 * 24 * time.Hour
)

var (
	ErrAddUser               = errors.New("add subscription failed")
	ErrUpdateUser            = errors.New("update user failed")
	ErrUserAlreadyReadItem   = errors.New("user already read this item")
	ErrUserAlreadyUnreadItem = errors.New("user already unread this item")
	ErrAlreadyFavorite       = errors.New("already favorited")
	ErrNotSubscribed         = errors.New("user not subscribed to feed")
	ErrInvalidUser           = errors.New("user data is invalid")
)

// NewUser creates a new user from the external provider details.
func NewUser(externalID, email, provider string) *User {
	ts := time.Now().UTC()
	return &User{
		CreatedAt:      ts,
		UpdatedAt:      ts,
		ExternalUserId: externalID,
		Email:          email,
		Provider:       provider,
		UserID:         NewID(UserPFX),
		Settings:       *NewUserSettings(),
	}
}

// Valid returns a boolean indicating whether the user data is valid. If not valid, it will also return a non-nil error
// that contains the validation issues.
func (u *User) Valid(_ context.Context) (bool, error) {
	valid, err := validation.ValidateStruct(u)
	switch {
	case err != nil:
		return false, fmt.Errorf("%w: %w", ErrInvalidUser, err)
	case !valid:
		return valid, ErrInvalidUser
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

// GetNickname retrieves the nickname of the user.
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

// GetSubscriptionMetadata retrieves a slice of the metadata for the user subscriptions.
func (u *User) GetSubscriptionMetadata() SubscriptionMetadataSlice {
	return u.Subscriptions
}

// IsSubscribedToFeed returns a boolean indicating whether the user is subscribed to a feed with the given id.
func (u *User) IsSubscribedToFeed(id FeedID) bool {
	idx := slices.IndexFunc(u.Subscriptions, func(e *SubscriptionMetadata) bool {
		return e.GetFeedID() == id
	})
	return idx != -1
}

// MarkSubscriptions marks user subscriptions with the given ids with the given mark.
func (u *User) MarkSubscriptions(mark Mark, ids ...SubscriptionID) {
	var markedAt time.Time
	if mark == MarkRead {
		// Set marked at to now when marking read.
		markedAt = time.Now().UTC()
	} else {
		// Set marked at to max history when marking unread.
		markedAt = u.GetMaxHistory()
	}
	for subscription := range slices.Values(u.GetSubscriptionMetadata().FilterByIDs(ids...)) {
		subscription.Mark(mark, markedAt)
	}
}

// MarkItems marks the given items in a user subscription the given mark.
func (u *User) MarkItems(mark Mark, subscriptionID SubscriptionID, itemIDs ...ItemID) {
	idx := slices.IndexFunc(u.Subscriptions, func(e *SubscriptionMetadata) bool {
		return e.GetID() == subscriptionID
	})
	if idx != -1 {
		switch mark {
		case MarkRead:
			u.Subscriptions[idx].MarkItemsRead(itemIDs...)
		case MarkUnread:
			u.Subscriptions[idx].MarkItemsUnread(itemIDs...)
		}
		u.Subscriptions[idx].UpdatedAt = time.Now().UTC()
	}
}

// AddSubscriptions adds to the user subscriptions the given metadata.
func (u *User) AddSubscriptions(subscriptions ...*SubscriptionMetadata) {
	for s := range slices.Values(subscriptions) {
		u.Subscriptions = append(u.Subscriptions, s)
	}
}

// UpdateSubscription replaces existing subscription metadata in the user object with the given data.
func (u *User) UpdateSubscription(update *SubscriptionMetadata) error {
	idx := slices.IndexFunc(u.Subscriptions, func(e *SubscriptionMetadata) bool {
		return e.GetID() == update.GetID()
	})
	if idx != -1 {
		u.Subscriptions[idx] = update
		return nil
	}
	return ErrNotSubscribed
}

// RemoveSubscriptions removes the user subscriptions with the matching id.
func (u *User) RemoveSubscriptions(ids ...SubscriptionID) {
	u.Subscriptions = slices.Collect(
		FilterSlice(u.Subscriptions, func(e *SubscriptionMetadata) bool {
			return !slices.Contains(ids, e.GetID())
		}),
	)
}

// GetAllFavorites returns the slice of user favorites.
func (u *User) GetAllFavorites() FavoritesSlice {
	return u.Favorites
}

// GetFavorite returns the user favorite with the given ID or nil if there is no favorite.
func (u *User) GetFavorite(id string) *Favorite {
	idx := slices.IndexFunc(u.Favorites, func(f *Favorite) bool {
		return f.GetID() == id
	})
	if idx != -1 {
		return u.Favorites[idx]
	}
	return nil
}

// IsFavorite returns a boolean indicating whether the user marked an object with the given ID as a favorite.
func (u *User) IsFavorite(id string) bool {
	return slices.ContainsFunc(u.Favorites, func(f *Favorite) bool {
		return f.GetID() == id
	})
}

// AddFavoriteSubscription creates a new favorite subscription for the user.
func (u *User) AddFavoriteSubscription(id SubscriptionID, nickname string) error {
	if u.GetAllFavorites().FilterByType(FavoriteTypeSubscription).HasFavorite(id) {
		return ErrAlreadyFavorite
	}
	fav := newFavorite(FavoriteTypeSubscription, nickname)
	fav.SetID(id)
	u.Favorites = append(u.Favorites, fav)
	return nil
}

// AddFavoriteArticle creates a new favorite article for the user.
func (u *User) AddFavoriteArticle(nickname string, article *Article) error {
	if u.GetAllFavorites().FilterByType(FavoriteTypeArticle).HasFavorite(article.GetID()) {
		return ErrAlreadyFavorite
	}
	fav := newFavorite(FavoriteTypeArticle, nickname)
	fav.SetID(article.GetID())
	err := fav.ObjectData.FromFavoriteArticle(FavoriteArticle{
		SubscriptionID: article.GetSubscriptionID(),
	})
	if err != nil {
		return fmt.Errorf("could not create favorite article: %w", err)
	}
	u.Favorites = append(u.Favorites, fav)
	return nil
}

// AddFavoriteSearch creates a new favorite search for the user.
func (u *User) AddFavoriteSearch(nickname string, search *SearchRequest) error {
	id := search.ID()
	if id == "" {
		return fmt.Errorf("%w: cannot generate search id", ErrUpdateUser)
	}
	if u.GetAllFavorites().FilterByType(FavoriteTypeSearch).HasFavorite(id) {
		return ErrAlreadyFavorite
	}
	fav := newFavorite(FavoriteTypeSearch, nickname)
	fav.SetID(id)
	err := fav.ObjectData.FromFavoriteSearch(*search)
	if err != nil {
		return fmt.Errorf("could not create favorite search: %w", err)
	}
	u.Favorites = append(u.Favorites, fav)
	return nil
}

func (u *User) UpdateFavoriteSearch(nickname string, search *SearchRequest) error {
	// Find the index of the existing favorite search entry in the user favorites.
	idx := slices.IndexFunc(u.GetAllFavorites(), func(f *Favorite) bool {
		return f.Nickname == nickname
	})
	// Replace the existing favorite entry.
	if idx != -1 {
		fav := newFavorite(FavoriteTypeSearch, nickname)
		fav.SetID(search.ID())
		err := fav.ObjectData.FromFavoriteSearch(*search)
		if err != nil {
			return fmt.Errorf("could not create favorite search: %w", err)
		}
		u.Favorites[idx] = fav
	}
	return nil
}

// RemoveFavorite removes the favorite with the given id from the user.
func (u *User) RemoveFavorite(id string) {
	favorites := slices.DeleteFunc(u.Favorites, func(f *Favorite) bool {
		return f.GetID() == id
	})
	u.Favorites = favorites
}

// NewUserSettings returns a new instance of the default user settings.
func NewUserSettings() *UserSettings {
	return &UserSettings{
		Theme:            DefaultUserTheme,
		ShowOnboarding:   true,
		ShowUnreadCounts: true,
		MaxHistory:       DefaultMaxHistory.String(),
	}
}

// Valid returns a boolean indicating if the UserSettings contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *UserSettings) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(s)
	if err != nil || !valid {
		return false, fmt.Errorf("%w: %w", ErrInvalidUser, err)
	}
	// Make sure max history is a valid duration value.
	maxHistory, err := time.ParseDuration(s.MaxHistory)
	if err != nil {
		return false, fmt.Errorf("%w: max history is invalid", ErrInvalidUser)
	}
	// Make sure max history is not greater than default max history.
	if maxHistory > DefaultMaxHistory {
		return false, fmt.Errorf("%w: max history is invalid", ErrInvalidUser)
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

//
// Favorites.
//

func newFavorite(favType FavoriteType, nickname string) *Favorite {
	return &Favorite{
		CreatedAt: time.Now().UTC(),
		Type:      favType,
		Nickname:  nickname,
	}
}

func (f *Favorite) GetID() string {
	return f.ObjectID
}

func (f *Favorite) SetID(id string) {
	f.ObjectID = id
}

type FavoritesSlice []*Favorite

// FilterByType will return a new slice filtered to the given favorite type.
func (f FavoritesSlice) FilterByType(favoriteType FavoriteType) FavoritesSlice {
	return slices.Collect(FilterSlice(f, func(f *Favorite) bool {
		return f.Type == favoriteType
	}))
}

// HasFavorite returns a boolean indicating whether the user has a favorite with the given object id.
func (f FavoritesSlice) HasFavorite(id string) bool {
	return slices.ContainsFunc(f, func(f *Favorite) bool {
		return f.GetID() == id
	})
}

// Get retrieves the favorite with the given id.
func (f FavoritesSlice) Get(id string) *Favorite {
	idx := slices.IndexFunc(f, func(f *Favorite) bool {
		return f.GetID() == id
	})
	if idx != -1 {
		return f[idx]
	}
	return nil
}

//
// SubscriptionMetadata
//

func (s *SubscriptionMetadata) GetID() SubscriptionID {
	return s.SubscriptionID
}

func (s *SubscriptionMetadata) GetFeedID() FeedID {
	return s.FeedID
}

// MarkRead will mark the subscription as read. This involves setting the MarkedRead field to the given value and
// removing any individual unread/read items.
func (s *SubscriptionMetadata) Mark(mark Mark, markedAt time.Time) {
	switch mark {
	case MarkRead:
		s.MarkedReadAt = markedAt
	case MarkUnread:
		s.MarkedReadAt = markedAt
	}
	s.ItemStates = nil
}

// GetUnreadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as unread.
func (s *SubscriptionMetadata) GetUnreadItems() []ItemID {
	ids := make([]ItemID, 0, len(s.ItemStates))
	for id, state := range s.ItemStates {
		if !state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetReadItems retrieves a list of ItemIDs for the subscription feed that
// user has explicitly marked as read.
func (s *SubscriptionMetadata) GetReadItems() []ItemID {
	ids := make([]ItemID, 0, len(s.ItemStates))
	for id, state := range s.ItemStates {
		if state.Read {
			ids = append(ids, id)
		}
	}
	return ids
}

// GetItemState retrieves the item state (read/unread/saved) from the
// subscription. By default it will return unread unless the user has explicitly
// marked or saved the item.
func (s *SubscriptionMetadata) GetItemState(id ItemID) *ArticleState {
	// Retrieve any explicitly set state of the item.
	if state, found := s.ItemStates[id]; found {
		return &state
	}
	// If an item doesn't have an explicit state, its state should reflect the subscription state.
	return &ArticleState{
		Read:      false,
		UpdatedAt: s.UpdatedAt,
	}
}

func (s *SubscriptionMetadata) SetItemState(id ItemID, state *ArticleState) {
	if s.ItemStates == nil {
		s.ItemStates = make(map[ItemID]ArticleState)
	}
	s.ItemStates[id] = *state
}

// MarkItemsRead will mark the given items as read for the subscription.
func (s *SubscriptionMetadata) MarkItemsRead(ids ...ItemID) {
	for id := range slices.Values(ids) {
		state := s.GetItemState(id)
		if state == nil {
			state = &ArticleState{}
		}
		state.MarkRead(time.Now().UTC())
		s.SetItemState(id, state)
	}
}

// MarkItemsUnread will mark the given items as unread for the subscription.
func (s *SubscriptionMetadata) MarkItemsUnread(ids ...ItemID) {
	for id := range slices.Values(ids) {
		state := s.GetItemState(id)
		if state == nil {
			state = &ArticleState{}
		}
		state.MarkUnread(time.Now().UTC())
		s.SetItemState(id, state)
	}
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *SubscriptionMetadata) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(s)
	if err != nil || !valid {
		return false, fmt.Errorf("subscription is invalid: %w", err)
	}
	return true, nil
}

// SubscriptionMetadataSlice is a slice of subscription metadata.
type SubscriptionMetadataSlice []*SubscriptionMetadata

// FilterByIDs returns a new slice containing the metadata for subscriptions with the given ids only.
func (s SubscriptionMetadataSlice) FilterByIDs(ids ...SubscriptionID) SubscriptionMetadataSlice {
	return slices.Collect(
		FilterSlice(s, func(e *SubscriptionMetadata) bool {
			return slices.Contains(ids, e.GetID())
		}),
	)
}

// FilterByFeedIDs returns a new slice containing the metadata for subscriptions with the given feed ids only. If no ids are
// provided, it returns the unfiltered slice.
func (s SubscriptionMetadataSlice) FilterByFeedIDs(ids ...FeedID) SubscriptionMetadataSlice {
	if len(ids) == 0 {
		return s
	}
	return slices.Collect(
		FilterSlice(s, func(e *SubscriptionMetadata) bool {
			return slices.Contains(ids, e.GetFeedID())
		}),
	)
}

// Search performs a case-insensitive substring search for the given text in the title and categories customisations for
// the subscriptions, returning a slice of those subscriptions that match.
func (s SubscriptionMetadataSlice) Search(text string) SubscriptionMetadataSlice {
	return slices.Collect(
		FilterSlice(s, func(e *SubscriptionMetadata) bool {
			return strings.Contains(strings.ToLower(e.Customisation.Nickname), strings.ToLower(text)) ||
				slices.ContainsFunc(e.Customisation.Categories, func(e Category) bool {
					return strings.Contains(strings.ToLower(e), strings.ToLower(text))
				})
		}),
	)
}

// GetFeedIDs returns the feed ids for all subscription states in the slice.
func (s SubscriptionMetadataSlice) GetFeedIDs() []FeedID {
	ids := make([]FeedID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetFeedID())
	}
	return ids
}

// GetIDs returns the subscription ids for all subscription states in the slice.
func (s SubscriptionMetadataSlice) GetIDs() []SubscriptionID {
	ids := make([]SubscriptionID, 0, len(s))
	for state := range slices.Values(s) {
		ids = append(ids, state.GetID())
	}
	return ids
}

// GetByID retrieves a state by the subscription id from the slice.
func (s SubscriptionMetadataSlice) GetByID(id SubscriptionID) *SubscriptionMetadata {
	if idx := slices.IndexFunc(s, func(e *SubscriptionMetadata) bool {
		return e.GetID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// GetByFeedID retrieves a state by the FeedID from the slice.
func (s SubscriptionMetadataSlice) GetByFeedID(id FeedID) *SubscriptionMetadata {
	if idx := slices.IndexFunc(s, func(e *SubscriptionMetadata) bool {
		return e.GetFeedID() == id
	}); idx != -1 {
		return s[idx]
	}
	return nil
}

// SortByTitle sorts the slice of subscriptions by their title.
func (s SubscriptionMetadataSlice) SortByTitle() SubscriptionMetadataSlice {
	sort.Slice(s, func(i, j int) bool { return s[i].Customisation.Nickname < s[j].Customisation.Nickname })
	return s
}
