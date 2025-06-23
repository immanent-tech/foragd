// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
)

var (
	ErrNoUserCtx        = errors.New("no valid user in context")
	ErrGetUserFailed    = errors.New("get user request failed")
	ErrCreateUserFailed = errors.New("create user request failed")
	ErrNoUser           = errors.New("no user found")
)

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

	user := &models.User{
		UserID:     userID,
		CreatedAt:  created,
		MaxHistory: models.DefaultMaxHistory.String(),
	}

	err := CreateDoc(ctx, e.GetAPI(), index, userID, user)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAPIRequestFailed, err)
	}

	return nil
}
