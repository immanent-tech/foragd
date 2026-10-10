/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

//go:generate go tool mockery
package handlers_test

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"

	"github.com/immanent-tech/foragd/server/handlers"
)

func TestMain(m *testing.M) {
	if err := godotenv.Load("../../.env.development"); err != nil {
		// Not fatal — CI often sets real env vars instead of a .env file
		slog.Warn("no .env file found, using existing environment")
	}
	os.Exit(m.Run())
}

func setupManager(t testing.TB) *handlers.Manager {
	t.Helper()
	return &handlers.Manager{
		AppConfig: &MoqAppConfig{
			GetBaseURLFunc: func() *url.URL {
				baseURL, _ := url.Parse("http://localhost")
				return baseURL
			},
			IsProductionFunc: func() bool {
				return false
			},
		},
		SessionMgr: &MoqSessionManager{
			GetFunc: func(ctx context.Context, key string) any {
				return nil
			},
		},
		Breadcrumbs: &MoqBreadcrumbs{},
	}
}

func withURLParams(t testing.TB, ctx context.Context, params map[string]string) context.Context {
	t.Helper()
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}
