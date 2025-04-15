// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"
)

var ErrNoAuthProvider = errors.New("no auth provider found")

const providerCtxKey contextKey = "provider"

type contextKey string

// ProviderToCtx stores the provider value in the context.
func ProviderToCtx(ctx context.Context, provider string) context.Context {
	slogctx.FromCtx(ctx).Debug("Storing auth provider in context.", slog.String("provider", provider))
	return context.WithValue(ctx, providerCtxKey, provider)
}

// ProviderFromCtx retrieves the provider value from the context.
func ProviderFromCtx(ctx context.Context) (string, bool) {
	provider, found := ctx.Value(providerCtxKey).(string)
	if !found {
		return "", false
	}
	return provider, true
}

func GetAuthProvider(req *http.Request) (string, error) {
	provider, found := ProviderFromCtx(req.Context())
	if !found {
		return "", ErrNoAuthProvider
	}
	slogctx.FromCtx(req.Context()).Debug("Retrieved auth provider from context.", slog.String("provider", provider))
	return provider, nil
}
