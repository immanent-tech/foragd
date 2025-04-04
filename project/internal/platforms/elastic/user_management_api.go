// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/session"
)

var (
	ErrNoUserCtx        = errors.New("no valid user in context")
	ErrGetUserFailed    = errors.New("get user request failed")
	ErrCreateUserFailed = errors.New("create user request failed")
	ErrNoUser           = errors.New("no user found")
)

// GetUser fetches the user record from Elasticsearch.
func (e *ElasticAPI) GetUser(ctx context.Context) (*models.User, error) {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return nil, errors.Join(ErrGetFailed, ErrFetchCtx)
	}

	userID, err := session.UserID(ctx)
	if err != nil {
		return nil, errors.Join(ErrGetFailed, ErrNoUserCtx, err)
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
func (e *ElasticAPI) AddUser(ctx context.Context, userID models.UserID) error {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrCreateUserFailed, ErrFetchCtx)
	}

	created := time.Now().UTC()

	logging.FromContext(ctx).Debug("adding user.", slog.Any("user", &models.User{
		UserID:    userID,
		CreatedAt: created,
	}))

	resp, err := NewDocCreateRequest(e.GetAPI(),
		index,
		userID,
		&models.User{
			UserID:    userID,
			CreatedAt: created,
		},
		refresh.True).
		Do(ctx)
	if err != nil {
		return errors.Join(ErrCreateUserFailed, err)
	}

	logging.FromContext(ctx).Debug("Added user.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))

	return nil
}

func (e *ElasticAPI) UpdateUser(ctx context.Context, id models.UserID, partialUpdate map[string]any) error {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrUpdateFailed, ErrFetchCtx)
	}

	// Updated the `updated_at` timestamp.
	partialUpdate["updated_at"] = time.Now().UTC()

	// Update the user in the store with the new list of read items.
	resp, err := NewDocUpdateRequest(e.GetAPI(), index, id,
		WithPartialDocUpdate(partialUpdate),
	).Do(ctx)
	if err != nil {
		return errors.Join(ErrUpdateFailed, err)
	}

	slog.Debug("Updated user.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))

	return nil
}
