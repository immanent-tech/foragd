/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestManager_HandleMarkSubscription(t *testing.T) {
	tests := []struct {
		name           string
		subSvc         handlers.SubscriptionsService
		prev           string
		breadcrumbs    handlers.Breadcrumbs
		mark           models.Mark
		subscriptionID models.SubscriptionID
		ctxSetup       func(ctx context.Context) context.Context
		want           int
	}{
		{
			name: "mark read",
			subSvc: &MoqSubscriptionsService{
				MarkSubscriptionsFunc: func(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) error {
					return nil
				},
			},
			subscriptionID: "sub_read",
			prev:           "/subscriptions",
			want:           http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				ctx = models.UserToCtx(ctx, &models.User{})
				ctx = models.SubscriptionToCtx(ctx, &models.Subscription{
					SubscriptionID: "sub_read",
				})
				return ctx
			},
		},
		{
			name: "mark unread",
			subSvc: &MoqSubscriptionsService{
				MarkSubscriptionsFunc: func(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) error {
					return nil
				},
			},
			subscriptionID: "sub_unread",
			prev:           "/subscriptions",
			want:           http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				ctx = models.UserToCtx(ctx, &models.User{})
				ctx = models.SubscriptionToCtx(ctx, &models.Subscription{
					SubscriptionID: "sub_unread",
				})
				return ctx
			},
		},
		{
			name: "no subscription",
			want: http.StatusNotFound,
			ctxSetup: func(ctx context.Context) context.Context {
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
		},
		{
			name: "mark read (articles)",
			subSvc: &MoqSubscriptionsService{
				MarkSubscriptionsFunc: func(ctx context.Context, mark models.Mark, subscriptionIDs ...models.SubscriptionID) error {
					return nil
				},
			},
			subscriptionID: "sub_read",
			prev:           "/articles",
			want:           http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				ctx = models.UserToCtx(ctx, &models.User{})
				ctx = models.SubscriptionToCtx(ctx, &models.Subscription{
					SubscriptionID: "sub_read",
				})
				return ctx
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			mgr.Breadcrumbs = &MoqBreadcrumbs{
				PreviousFunc: func(ctx context.Context) (*url.URL, bool) {
					prevURL, _ := url.Parse("/subscriptions")
					return prevURL, true
				},
			}
			ctx := t.Context()
			if tt.ctxSetup != nil {
				ctx = tt.ctxSetup(ctx)
			}

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodPost,
				"/subscriptions/"+string(tt.subscriptionID)+"/"+string(tt.mark),
				nil,
			)
			req.Header.Set(htmx.HeaderRequest, "true")

			mgr.HandleMarkSubscription(tt.subSvc, tt.mark)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusOK {
				prevURL, ok := mgr.Breadcrumbs.Previous(ctx)
				require.True(t, ok, "check previous breadcrumbs")
				switch prevURL.String() {
				case "/subscriptions":
					assert.Equal(
						t,
						rec.Header().Get(htmx.HeaderReswap),
						"delete transition:true swap:300ms",
						"reswap header set",
					)
					assert.Equal(t, rec.Header().Get(htmx.HeaderRetarget), "#"+tt.subscriptionID, "retarget header set")
					assert.Equal(t, rec.Header().Get(htmx.HeaderTrigger), "masonry:update", "trigger header set")
				case "/articles":
					assert.Contains(t, rec.Header().Get(htmx.HeaderLocation), "#content", "location target set")
					assert.Contains(
						t,
						rec.Header().Get(htmx.HeaderLocation),
						"innerMorph transition:true",
						"location swap set",
					)
					assert.Contains(t, rec.Header().Get(htmx.HeaderLocation), "/subscriptions", "location path set")
				}
			}
		})
	}
}
