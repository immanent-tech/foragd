// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package session

import (
	"context"
	"fmt"
)

func UserID(ctx context.Context) (string, error) {
	tokens, err := GetTokens(ctx)
	if err != nil {
		return "", fmt.Errorf("could not retrieve user id: %w", err)
	}

	return tokens.IDToken.Subject, nil
}
