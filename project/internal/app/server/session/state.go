// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package session

import (
	"context"
)

func StoreState(ctx context.Context, state string) error {
	sessionManager.Put(ctx, stateSessionKey, state)

	return nil
}

func GetState(ctx context.Context) (string, error) {
	data := sessionManager.Get(ctx, stateSessionKey)
	preferences, ok := data.(string)

	switch {
	case data == nil:
		return "", ErrDataNotFound
	case ok:
		return preferences, nil
	default:
		return "", ErrInvalidData
	}
}
