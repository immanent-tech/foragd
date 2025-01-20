// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/refresh"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

var (
	ErrNoUserCtx        = errors.New("no valid user in context")
	ErrGetUserFailed    = errors.New("get user request failed")
	ErrCreateUserFailed = errors.New("create user request failed")
)

// GetUser fetches the user record from Elasticsearch.
func (c *Client) GetUser(ctx context.Context) (models.User, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return models.User{}, errors.Join(ErrNoUserCtx, err)
	}

	resp, err := c.NewGetRequest(schema.UsersSchemaPrefix, userID).Do(ctx)
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
	found, err := c.NewDocExistsRequest(schema.UsersSchemaPrefix, userID).Do(ctx)
	if err != nil {
		return false, errors.Join(ErrExistsFailed, err)
	}

	return found, nil
}

// AddUser creates a new user record.
func (c *Client) AddUser(ctx context.Context, userID models.UserID) error {
	resp, err := c.NewDocCreateRequest(
		schema.UsersSchemaPrefix,
		userID,
		&models.User{
			ID:        userID,
			CreatedAt: time.Now(),
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
