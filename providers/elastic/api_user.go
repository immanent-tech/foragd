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
	"github.com/immanent-tech/foragd/providers/elastic/schema"
)

// CreateUser creates a new user doc in Elasticsearch.
func (a *API) CreateUser(ctx context.Context, user *models.User) error {
	index := schema.UsersSchemaPrefix + schema.IndexWriteSuffix
	if err := CreateDoc(ctx, a, index, user.GetID(), user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUser retrieves the user doc with the given id.
func (a *API) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	index := schema.UsersSchemaPrefix + schema.IndexReadSuffix
	user, err := GetDoc[models.UserID, *models.User](ctx, a, index, id)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// DeleteUser removes the user doc with the given ID.
func (a *API) DeleteUser(ctx context.Context, id models.UserID) error {
	index := schema.UsersSchemaPrefix + schema.IndexWriteSuffix
	if err := DeleteDoc(ctx, a, index, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

// UpdateUser will apply the given updates to the user.
func (a *API) UpdateUser(ctx context.Context, userID models.UserID, updates map[string]any) error {
	index := schema.UsersSchemaPrefix + schema.IndexWriteSuffix
	updates["updated_at"] = time.Now().UTC()
	if err := UpdateDoc(ctx, a, index, userID, updates,
		WithRefresh("true"),
		WithRetryOnConflict(defaultRetries),
	); err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

// FindUserByExternalID will search for and return a user that matches the given external ID, if exists.
func (a *API) FindUserByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	index := schema.UsersSchemaPrefix + schema.IndexReadSuffix
	// Get the user.
	users, _, err := Search[*models.User](ctx, a, index, query.Term("external_user_id", externalID), 1,
		WithSortOptions[*search.Search, SearchRequest](&types.SortOptions{Doc_: types.NewScoreSort()}),
		WithTrackTotalHits(false),
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
