// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/joshuar/go-feed-me/internal/models"
)

type DB interface {
	GetUserByID(ctx context.Context, userID string) (*models.User, error)
}

func UserID(ctx context.Context) (string, error) {
	tokens, err := GetTokens(ctx)
	if err != nil {
		return "", fmt.Errorf("could not retrieve user id: %w", err)
	}

	return tokens.IDToken.Subject, nil
}

func ValidUser(ctx context.Context, db DB) (bool, error) {
	// Get the user tokens from the session storage.
	tokens, err := GetTokens(ctx)
	if err != nil {
		return false, fmt.Errorf("unable to get user details from session: %w", err)
	}
	// Ensure the user ID in the token matches a user in the database.
	_, err = db.GetUserByID(ctx, tokens.UserID())
	if err != nil {
		return false, fmt.Errorf("unable to get user details from session: %w", err)
	}

	return true, nil
}
