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

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
)

func TestManager_HandleListSubscriptionsGet(t *testing.T) {
	tests := []struct {
		name     string
		subSvc   handlers.SubscriptionsService
		ctxSetup func(ctx context.Context) context.Context
		want     int
	}{
		{
			name: "ok",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					return models.Subscriptions{&models.Subscription{}}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no subscriptions",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					return nil, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)

			mgr.HandleListSubscriptions(tt.subSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestManager_HandleListSubscriptionsPost(t *testing.T) {
	tests := []struct {
		name     string
		subSvc   handlers.SubscriptionsService
		ctxSetup func(ctx context.Context) context.Context
		want     int
	}{
		{
			name: "ok",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					return models.Subscriptions{&models.Subscription{}}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no subscriptions",
			subSvc: &MoqSubscriptionsService{
				GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
					return nil, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/subscriptions", nil)
			req.Header.Set(htmx.HeaderRequest, "true")

			mgr.HandleListSubscriptions(tt.subSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
