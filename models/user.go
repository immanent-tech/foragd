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
func (u *User) GetSubscriptions(options ...subscriptionFilterOption) Subscriptions {
	subscriptions := make(Subscriptions, 0, len(u.Subscriptions))
	for subscriptionData := range slices.Values(u.Subscriptions) {
		subscription := Subscription{
			CreatedAt:      subscriptionData.CreatedAt,
			Customisation:  subscriptionData.Customisation,
			Favorite:       u.IsFavorite(subscriptionData.GetID()),
			MarkedReadAt:   subscriptionData.MarkedReadAt,
			Settings:       subscriptionData.Settings,
			SubscriptionID: subscriptionData.GetID(),
			Type:           SubscriptionTypeFeed,
			UpdatedAt:      subscriptionData.UpdatedAt,
		}
		// ! No error check...
		subscription.Data.FromFeedSubscription(*subscriptionData) //nolint:errcheck
		subscriptions = append(subscriptions, &subscription)
	}
	// Apply filtering options.
	for option := range slices.Values(options) {
		subscriptions = option(subscriptions)
	}
	return subscriptions
}

type subscriptionFilterOption func(Subscriptions) Subscriptions

// FilterByIDs option will filter the subscriptions by the given IDs.
func FilterByIDs(ids ...SubscriptionID) subscriptionFilterOption {
	return func(s Subscriptions) Subscriptions {
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
	return func(s Subscriptions) Subscriptions {
		sort.Slice(s, func(i, j int) bool { return s[i].GetTitle() < s[j].GetTitle() })
		return s
	}
}

// IsSubscribedToFeed returns a boolean indicating whether the user is subscribed to a feed with the given id.
func (u *User) IsSubscribedToFeed(id FeedID) bool {
	idx := slices.IndexFunc(u.Subscriptions, func(e *FeedSubscription) bool {
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
	for subscription := range slices.Values(u.Subscriptions.FilterByIDs(ids...)) {
		subscription.Mark(mark, markedAt)
	}
}

// MarkItems marks the given items in a user subscription the given mark.
func (u *User) MarkItems(mark Mark, subscriptionID SubscriptionID, itemIDs ...ItemID) {
	idx := slices.IndexFunc(u.Subscriptions, func(e *FeedSubscription) bool {
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
func (u *User) AddSubscriptions(subscriptions ...*FeedSubscription) {
	for s := range slices.Values(subscriptions) {
		u.Subscriptions = append(u.Subscriptions, s)
	}
}

// UpdateSubscription replaces existing subscription metadata in the user object with the given data.
func (u *User) UpdateSubscription(update *FeedSubscription) error {
	idx := slices.IndexFunc(u.Subscriptions, func(e *FeedSubscription) bool {
		return e.GetID() == update.GetID()
	})
	if idx != -1 {
		u.Subscriptions[idx] = update
		return nil
	}
	return ErrUserNotSubscribed
}

// RemoveSubscriptions removes the user subscriptions with the matching id.
func (u *User) RemoveSubscriptions(ids ...SubscriptionID) {
	u.Subscriptions = slices.Collect(
		FilterSlice(u.Subscriptions, func(e *FeedSubscription) bool {
			return !slices.Contains(ids, e.GetID())
		}),
	)
}

// GetAllFavorites returns the slice of user favorites.
func (u *User) GetAllFavorites() Favorites {
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
		return ErrUserAlreadyFavorited
	}
	fav := newFavorite(FavoriteTypeSubscription, nickname)
	fav.SetID(id)
	u.Favorites = append(u.Favorites, fav)
	return nil
}

// AddFavoriteArticle creates a new favorite article for the user.
func (u *User) AddFavoriteArticle(nickname string, article *Article) error {
	if u.GetAllFavorites().FilterByType(FavoriteTypeArticle).HasFavorite(article.GetID()) {
		return ErrUserAlreadyFavorited
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
	id, err := search.ID()
	if id == "" {
		return fmt.Errorf("could not favorite search: %w", err)
	}
	if u.GetAllFavorites().FilterByType(FavoriteTypeSearch).HasFavorite(id) {
		return ErrUserAlreadyFavorited
	}
	fav := newFavorite(FavoriteTypeSearch, nickname)
	fav.SetID(id)
	err = fav.ObjectData.FromFavoriteSearch(*search)
	if err != nil {
		return fmt.Errorf("could not create favorite search: %w", err)
	}
	u.Favorites = append(u.Favorites, fav)
	return nil
}

// UpdateFavoriteSearch updates the details of a favorite search.
func (u *User) UpdateFavoriteSearch(nickname string, search *SearchRequest) error {
	// Find the index of the existing favorite search entry in the user favorites.
	idx := slices.IndexFunc(u.GetAllFavorites(), func(f *Favorite) bool {
		return f.Nickname == nickname
	})
	// Replace the existing favorite entry.
	if idx != -1 {
		fav := newFavorite(FavoriteTypeSearch, nickname)
		id, err := search.ID()
		if err != nil {
			return fmt.Errorf("could not update favorite search: %w", err)
		}
		fav.SetID(id)
		err = fav.ObjectData.FromFavoriteSearch(*search)
		if err != nil {
			return fmt.Errorf("could not update favorite search: %w", err)
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

// GetID returns the ID of the favorite.
func (f *Favorite) GetID() string {
	return f.ObjectID
}

// SetID sets the ID of a favorite.
func (f *Favorite) SetID(id string) {
	f.ObjectID = id
}

// Favorites is a slice of favorites.
type Favorites []*Favorite

// FilterByType will return a new slice filtered to the given favorite type.
func (f Favorites) FilterByType(favoriteType FavoriteType) Favorites {
	return slices.Collect(FilterSlice(f, func(f *Favorite) bool {
		return f.Type == favoriteType
	}))
}

// HasFavorite returns a boolean indicating whether the user has a favorite with the given object id.
func (f Favorites) HasFavorite(id string) bool {
	return slices.ContainsFunc(f, func(f *Favorite) bool {
		return f.GetID() == id
	})
}

// Get retrieves the favorite with the given id.
func (f Favorites) Get(id string) *Favorite {
	idx := slices.IndexFunc(f, func(f *Favorite) bool {
		return f.GetID() == id
	})
	if idx != -1 {
		return f[idx]
	}
	return nil
}
