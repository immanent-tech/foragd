// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc"

	"github.com/joshuar/go-feed-me/internal/models"
)

func StoreTokens(ctx context.Context, idToken *oidc.IDToken, accessToken string) error {
	tokens := models.Tokens{
		IDToken:     idToken,
		AccessToken: accessToken,
	}

	if err := tokens.DecodeClaims(); err != nil {
		return fmt.Errorf("cannot store tokens: %w", err)
	}

	sessionManager.Put(ctx, profileSessionKey, tokens)

	return nil
}

func GetTokens(ctx context.Context) (*models.Tokens, error) {
	data := sessionManager.Get(ctx, profileSessionKey)
	tokens, ok := data.(models.Tokens)

	switch {
	case data == nil:
		return nil, ErrDataNotFound
	case ok:
		return &tokens, nil
	default:
		return nil, ErrInvalidData
	}
}
