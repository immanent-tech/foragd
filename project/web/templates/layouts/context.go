// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package layouts

import (
	"context"

	"github.com/joshuar/go-feed-me/internal/models"
)

type contextKey string

const (
	userSignupCtxKey contextKey = "usersignup"
)

func UserSignupToCtx(ctx context.Context, signup *models.UserSignup) context.Context {
	return context.WithValue(ctx, userSignupCtxKey, signup)
}

func UserSignupFromCtx(ctx context.Context) *models.UserSignup {
	signup, ok := ctx.Value(userSignupCtxKey).(*models.UserSignup)
	if !ok {
		return nil
	}

	return signup
}
