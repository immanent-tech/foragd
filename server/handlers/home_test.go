/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
	"github.com/immanent-tech/foragd/service"
)

func SetupManager(t testing.TB) *handlers.Manager {
	t.Helper()
	return &handlers.Manager{
		AppConfig:   &MoqAppConfig{},
		SessionMgr:  &MoqSessionManager{},
		Breadcrumbs: &MoqBreadcrumbs{},
	}
}

func TestHandleHome(t *testing.T) {
	tests := []struct {
		name        string
		homepageSvc handlers.HomePageService
		reqSetup    func(ctx context.Context) *http.Request
		ctxSetup    func(ctx context.Context) context.Context
		want        int
	}{
		{
			name:        "new user",
			homepageSvc: &service.Home{},
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
			reqSetup: func(ctx context.Context) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/home", nil)
				return req.WithContext(ctx)
			},
			want: http.StatusOK,
		},
		{
			name:        "existing user, no subscriptions",
			homepageSvc: &service.Home{},
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{Settings: models.UserSettings{ShowOnboarding: false}})
			},
			reqSetup: func(ctx context.Context) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/home", nil)
				return req.WithContext(ctx)
			},
			want: http.StatusOK,
		},
		{
			name:        "no user",
			homepageSvc: &service.Home{},
			reqSetup: func(ctx context.Context) *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/home", nil)
				return req.WithContext(ctx)
			},
			want: http.StatusSeeOther,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := SetupManager(t)
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}
			req := tt.reqSetup(ctx)
			rec := httptest.NewRecorder()

			mgr.HandleHome(tt.homepageSvc)(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
