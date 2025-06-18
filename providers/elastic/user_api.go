// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
)

var (
	ErrNoUserCtx        = errors.New("no valid user in context")
	ErrGetUserFailed    = errors.New("get user request failed")
	ErrCreateUserFailed = errors.New("create user request failed")
	ErrNoUser           = errors.New("no user found")
)

// GetUser fetches the user record from Elasticsearch.
func (e *API) GetUser(ctx context.Context, userID models.UserID) (*models.User, error) {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrGetFailed, ErrFetchCtx)
	}

	resp, err := NewGetRequest(e.GetAPI(), index, userID).Do(ctx)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, err)
	}

	if !resp.Found {
		return nil, ErrNoUser
	}

	user, err := ExtractSource[models.User](resp.Source_)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, err)
	}

	return &user, nil
}

// UserExists checks if a user record exists in Elasticsearch for the given user ID.
func UserExists(ctx context.Context, api *typedapi.API, userID models.UserID) (bool, error) {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return false, errors.Join(ErrExistsFailed, ErrFetchCtx)
	}

	found, err := NewDocExistsRequest(api, index, userID).Do(ctx)
	if err != nil {
		return false, errors.Join(ErrExistsFailed, err)
	}

	return found, nil
}

// AddUser creates a new user record.
func (e *API) AddUser(ctx context.Context, userID models.UserID) error {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrCreateUserFailed, ErrFetchCtx)
	}

	created := time.Now().UTC()

	slogctx.FromCtx(ctx).Debug("adding user.", slog.Any("user", &models.User{
		UserID:    userID,
		CreatedAt: created,
	}))

	resp, err := NewDocCreateRequest(e.GetAPI(),
		index,
		userID,
		&models.User{
			UserID:     userID,
			CreatedAt:  created,
			MaxHistory: models.DefaultMaxHistory.String(),
		},
		refresh.True).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}

	slogctx.FromCtx(ctx).Debug("Added user.",
		slog.String("result", resp.Result.String()),
	)

	return nil
}

func (a *API) UpdateUser(ctx context.Context, updates map[string]any) error {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return fmt.Errorf("user update failed: %w", ErrNoUserCtx)
	}
	index := UserIndexFromCtx(ctx)

	if err := UpdateDoc(ctx, a.GetAPI(), index, user.GetID(), updates); err != nil {
		return fmt.Errorf("user update failed: %w", err)
	}
	return nil
}
