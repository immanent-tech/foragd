// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/stripe/stripe-go/v83"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/client"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// DefaultUserTheme is the default theme for the app.
	DefaultUserTheme = "greenhouse"
	// DefaultMaxHistory is a default maximum history value for when the user has not specified one.
	DefaultMaxHistory = 31 * 24 * time.Hour
	// DefaultUpdateInterval is the default interval on which to check for updates.
	DefaultUpdateInterval = 5 * time.Minute
	// MaxSubscriptions is the maxiumum number of subscriptions a user can have.
	MaxSubscriptions = 3000
)

var (
	ErrUserAlreadyFavorited = errors.New("already a favorite")
)

// GetUserByExternalID will search for and return a user that matches the given external ID, if exists.
func GetUserByExternalID(ctx context.Context, externalID string) (*User, error) {
	// Get the user.
	users, _, err := elastic.Search[*User](ctx, schema.UsersIndexRO, query.Term("external_user_id", externalID), 1,
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		elastic.WithTrackTotalHits(false),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("find user by external id: %w", err)
	case len(users) == 0:
		return nil, fmt.Errorf("find user by external id: %w", ErrNotFound)
	default:
		return users[0], nil
	}
}

// GetUserBySubscriptionEmail will retrieve a user from their emails.
func GetUserBySubscriptionEmail(ctx context.Context, emails ...string) (*User, error) {
	// Get the user.
	users, _, err := elastic.Search[*User](
		ctx,
		schema.UsersIndexRO,
		query.Terms("settings.subscription_email", emails...),
		1,
		elastic.WithSortOptions[*search.Search, elastic.SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		elastic.WithTrackTotalHits(false),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("find user by external id: %w", err)
	case len(users) == 0:
		return nil, fmt.Errorf("find user by external id: %w", ErrNotFound)
	default:
		return users[0], nil
	}
}

// GetUser retrieves the user doc with the given id.
func GetUser(ctx context.Context, id UserID) (*User, error) {
	user, err := elastic.GetDoc[UserID, *User](ctx, schema.UsersIndexRO, id)
	if err != nil || user == nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// UpdateUser will apply the given updates to the user.
func UpdateUser(ctx context.Context, userID UserID, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	if err := elastic.UpdateDoc(ctx, schema.UsersIndexRW, userID, updates,
		elastic.WithRefresh("true"),
		elastic.WithRetryOnConflict(client.DefaultRequestRetries),
	); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	slogctx.FromCtx(ctx).Info("User object updated.")
	return nil
}

// Valid returns a boolean indicating whether the user data is valid. If not valid, it will also return a non-nil error
// that contains the validation issues.
func (u *User) Valid(_ context.Context) error {
	if err := validation.Validate.Struct(u); err != nil {
		return fmt.Errorf("user data is invalid: %w", err)
	}
	return nil
}

// GetID returns the ID for the user.
func (u *User) GetID() UserID {
	return u.UserID
}

// GetExternalID returns the backend ID for the user.
func (u *User) GetExternalID() UserID {
	return u.ExternalUserID
}

// GetAvatar retrieves the URL to the image to represent the user.
func (u *User) GetAvatar() string {
	if u.AvatarURL != nil {
		return *u.AvatarURL
	}
	return ""
}

// GetNickname retrieves the nickname of the user.
func (u *User) GetNickname() string {
	return u.Nickname
}

// GetEmail retrieves the email of the user.
func (u *User) GetEmail() string {
	return u.Email
}

// GetMaxHistory returns a timestamp in the past from which the user can view items. If there is an issue retrieving and
// parsing the value from the user's metadata, it will default to using the lowest plan max history.
func (u *User) GetMaxHistory() time.Time {
	if u.GetSettings().MaxViewHistory == 0 {
		return time.Now().Add(-DefaultMaxHistory)
	}

	return time.Now().Add(-u.GetSettings().MaxViewHistory)
}

// GetUpdatesFrequency returns a duration on which the user will see new updates. If there is an issue retrieving and
// parsing the value from the user's metdata, it will use the lowest plan updates frequency.
func (u *User) GetUpdatesFrequency() time.Duration {
	if u.GetSettings().UpdatesInterval == 0 {
		return DefaultUpdateInterval
	}
	return u.GetSettings().UpdatesInterval
}

// OnTrial returns a boolean indicating whether the user is currently in a trial period and if so, a timestamp
// indicating when the trial will end.
func (u *User) OnTrial() (bool, time.Time) {
	if u.Subscription != nil {
		if *u.Subscription.PlanStatus == stripe.SubscriptionStatusTrialing {
			return true, *u.Subscription.TrialEnd
		}
	}
	return false, time.Time{}
}

// Cancelled returns a boolean indicating whether the user has cancelled their subscription plan and if so, a timestamp
// indicating when the cancellation will apply.
func (u *User) Cancelled() (bool, time.Time) {
	if u.Subscription != nil {
		if *u.Subscription.PlanStatus == stripe.SubscriptionStatusCanceled {
			if u.Subscription.CancelAt == nil {
				return true, time.Now().UTC()
			}
			if u.Subscription.CancelAt.After(time.Now().UTC()) {
				return true, *u.Subscription.CancelAt
			}
		}
	}
	return false, time.Time{}
}

// Active returns a boolean indicating whether the user is "active", which means a paying customer with no payment
// issues or customer currently on a trial.
func (u *User) Active() bool {
	// ! Uncomment after beta.
	// if u.Metadata.PlanStatus == stripe.SubscriptionStatusActive {
	// 	return true
	// }
	// if trial, _ := u.OnTrial(); trial {
	// 	return true
	// }
	// return false
	return true
}

// GetSubscriptionPlan returns the name of the subscription plan of the user.
func (u *User) GetSubscriptionPlan() string {
	if u.Subscription.Plan != nil {
		return *u.Subscription.Plan
	}
	return ""
}

// GetSettings returns the user's settings. If the user has no settings (i.e. new user), default settings will be
// returned.
func (u *User) GetSettings() *UserSettings {
	return &u.Settings
}

// Valid returns a boolean indicating if the UserSettings contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *UserSettings) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("invalid user settings: %w", err)
	}
	return nil
}

// Sanitise will sanitise UserSettings values.
func (s *UserSettings) Sanitise() error {
	// Multiply parsed MaxViewHistory by hours to get nanoseconds.
	s.MaxViewHistory = s.MaxViewHistory * time.Hour
	return nil
}

// Valid returns a boolean indicating if the UserSettings contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *UserMetadata) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("invalid user metadata: %w", err)
	}
	return nil
}

// GetUserTheme returns the current user's theme or the default theme if no user theme is set.
func GetUserTheme(ctx context.Context) string {
	user := UserFromCtx(ctx)
	if user == nil {
		return DefaultUserTheme
	}
	if theme := user.GetSettings().Theme; theme != "" {
		return theme
	}
	return DefaultUserTheme
}

// Valid returns a boolean indicating if the Subscription contains valid data (true). If it contains invalid data
// (false) a non-nil error is also returned which contains validation issues.
func (s *EditUserRequest) Valid() error {
	if err := validation.Validate.Struct(s); err != nil {
		return fmt.Errorf("request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the user input for a SubscriptionCustomisation.
func (s *EditUserRequest) Sanitise() error {
	s.Nickname = validation.SanitizeString(s.Nickname)
	s.Email = validation.SanitizeString(s.Email)
	return nil
}

// Valid returns a boolean indicating whether the ChangePasswordRequest contains valid data.
func (r *ChangePasswordRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the user input for a ChangePasswordRequest.
func (r *ChangePasswordRequest) Sanitise() error {
	return nil
}
