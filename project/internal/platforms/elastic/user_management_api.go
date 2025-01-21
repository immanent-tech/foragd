// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"

	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/models"
)

var (
	ErrNoUserCtx        = errors.New("no valid user in context")
	ErrGetUserFailed    = errors.New("get user request failed")
	ErrCreateUserFailed = errors.New("create user request failed")
)

// GetUser fetches the user record from Elasticsearch.
func (c *Client) GetUser(ctx context.Context) (models.User, error) {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return models.User{}, errors.Join(ErrGetFailed, ErrNoIndexInCtx)
	}

	userID, err := session.UserID(ctx)
	if err != nil {
		return models.User{}, errors.Join(ErrGetFailed, ErrNoUserCtx, err)
	}

	resp, err := c.NewGetRequest(index, userID).Do(ctx)
	if err != nil {
		return models.User{}, errors.Join(ErrGetFailed, err)
	}

	user, err := ExtractSource[models.User](resp.Source_)
	if err != nil {
		return models.User{}, errors.Join(ErrGetFailed, err)
	}

	return user, nil
}

// UserExists checks if a user record exists in Elasticsearch for the given user ID.
func (c *Client) UserExists(ctx context.Context, userID models.UserID) (bool, error) {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return false, errors.Join(ErrExistsFailed, ErrNoIndexInCtx)
	}

	found, err := c.NewDocExistsRequest(index, userID).Do(ctx)
	if err != nil {
		return false, errors.Join(ErrExistsFailed, err)
	}

	return found, nil
}

// AddUser creates a new user record.
func (c *Client) AddUser(ctx context.Context, userID models.UserID) error {
	index := UserIndexFromCtx(ctx)
	if index == "" {
		return errors.Join(ErrCreateUserFailed, ErrNoIndexInCtx)
	}

	createdAt := time.Now().UTC()
	c.Logger.Debug("adding user.", slog.Any("user", &models.User{
		ID:        userID,
		CreatedAt: &createdAt,
	}))

	resp, err := c.NewDocCreateRequest(
		index,
		userID,
		&models.User{
			ID:        userID,
			CreatedAt: &createdAt,
		},
		refresh.True).
		Do(ctx)
	if err != nil {
		return errors.Join(ErrCreateUserFailed, err)
	}

	c.Logger.Debug("Added user.",
		slog.String("result", resp.Result.String()),
		slog.Int64("version", resp.Version_))

	return nil
}
