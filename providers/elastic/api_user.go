// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// UserExists checks if a user doc with the given ID exists.
func (a *API) UserExists(ctx context.Context, id models.UserID) (bool, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return false, fmt.Errorf("user exists: %w", ErrNoIndexInCtx)
	}
	found, err := exists(ctx, a.TypedClient, index, id)
	if err != nil {
		return false, fmt.Errorf("user exists: %w", err)
	}
	return found, nil
}

// CreateUser creates a new user doc in Elasticsearch.
func (a *API) CreateUser(ctx context.Context, user *models.User) error {
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("create user: %w", ErrNoIndexInCtx)
	}
	err = CreateDoc(ctx, a.GetAPI(), index, user.GetID(), user)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUser retrieves the user doc with the given id.
func (a *API) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", ErrNoIndexInCtx)
	}
	user, err := GetDoc[models.UserID, *models.User](ctx, a.GetAPI(), index, id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// DeleteUser removes the user doc with the given ID.
func (a *API) DeleteUser(ctx context.Context, id models.UserID) error {
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("delete user: %w", ErrNoIndexInCtx)
	}
	err = DeleteDoc(ctx, a.GetAPI(), index, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// UpdateUser will apply the given updates to the user.
func (a *API) UpdateUser(ctx context.Context, userID models.UserID, updates map[string]any) error {
	updates["updated_at"] = time.Now().UTC()
	index, err := UserWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("update user: %w", ErrNoIndexInCtx)
	}
	err = UpdateDoc(ctx, a.GetAPI(), index, userID, updates,
		WithRefresh("true"),
		WithRetryOnConflict(defaultRetries),
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// FindUserByExternalID will search for and return a user that matches the given external ID, if exists.
func (a *API) FindUserByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	index, err := UserReadIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("find user by external id: %w", ErrNoIndexInCtx)
	}
	// Get the user.
	users, _, err := Search[*models.User](ctx, a.GetAPI(), index, query.Term("external_user_id", externalID), 1,
		WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		WithTrackTotalHits(false),
	)
	if err != nil {
		return nil, fmt.Errorf("find user by external id: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("find user by external id: %w", ErrNotFound)
	}
	return users[0], nil
}
