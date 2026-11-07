/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
)

func TestManager_HandleSearchSuggestions(t *testing.T) {
	tests := []struct {
		name    string
		subSvc  handlers.SubscriptionsService
		itemSvc handlers.ItemService
		ctx     func(ctx context.Context) context.Context
		want    int
	}{
		{
			name: "ok",
			subSvc: &MoqSubscriptionsService{
				GetSubscriptionSuggestionsFunc: func(ctx context.Context, text string, count int, ignoredSubscriptions []models.SubscriptionID) (models.Subscriptions, error) {
					return models.Subscriptions{
						&models.Subscription{},
					}, nil
				},
			},
			itemSvc: &MoqItemService{
				SuggestItemsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Items, error) {
					return models.Items{}, nil
				},
			},
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				search.Text = "test"
				ctx = handlers.SearchParamsToCtx(ctx, search)
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
			want: http.StatusOK,
		},
		{
			name: "no results",
			subSvc: &MoqSubscriptionsService{
				GetSubscriptionSuggestionsFunc: func(ctx context.Context, text string, count int, ignoredSubscriptions []models.SubscriptionID) (models.Subscriptions, error) {
					return nil, nil
				},
			},
			itemSvc: &MoqItemService{
				SuggestItemsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Items, error) {
					return nil, nil
				},
			},
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				search.Text = "test"
				ctx = handlers.SearchParamsToCtx(ctx, search)
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
			want: http.StatusOK,
		},
		{
			name: "no user",
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				search.Text = "test"
				ctx = handlers.SearchParamsToCtx(ctx, search)
				return ctx
			},
			want: http.StatusSeeOther,
		},
		{
			name: "no search text",
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				ctx = handlers.SearchParamsToCtx(ctx, search)
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
			want: http.StatusNoContent,
		},
		{
			name: "error",
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				search.Text = "test"
				ctx = handlers.SearchParamsToCtx(ctx, search)
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
			subSvc: &MoqSubscriptionsService{
				GetSubscriptionSuggestionsFunc: func(ctx context.Context, text string, count int, ignoredSubscriptions []models.SubscriptionID) (models.Subscriptions, error) {
					return nil, nil
				},
			},
			itemSvc: &MoqItemService{
				SuggestItemsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Items, error) {
					return nil, errors.New("failed!")
				},
			},
			want: http.StatusNoContent,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := tt.ctx(t.Context())

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/search/suggestions", nil)
			req.Header.Add(htmx.HeaderRequest, "true")

			mgr.HandleSearchSuggestions(tt.subSvc, tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestManager_HandleSearchResults(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		itemSvc handlers.ItemService
		ctx     func(ctx context.Context) context.Context
		htmx    bool
		want    int
	}{
		{
			name:   "ok",
			method: http.MethodGet,
			itemSvc: &MoqItemService{
				RetrieveItemsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Items, models.Pagination, error) {
					return models.Items{}, models.Pagination{}, nil
				},
				GetTopItemCategoriesForSearchResultsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Categories, error) {
					return models.Categories{}, nil
				},
			},
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				search.Text = "test"
				ctx = handlers.SearchParamsToCtx(ctx, search)
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
			want: http.StatusOK,
		},
		// {
		// 	name: "no results",
		// 	itemSvc: &MoqItemService{
		// 		SuggestItemsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Items, error) {
		// 			return nil, nil
		// 		},
		// 	},
		// 	ctx: func(ctx context.Context) context.Context {
		// 		search := models.NewSearchRequest()
		// 		search.Text = "test"
		// 		ctx = handlers.SearchParamsToCtx(ctx, search)
		// 		ctx = models.UserToCtx(ctx, &models.User{})
		// 		return ctx
		// 	},
		// 	want: http.StatusNoContent,
		// },
		{
			name:   "no user",
			method: http.MethodGet,
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				search.Text = "test"
				ctx = handlers.SearchParamsToCtx(ctx, search)
				return ctx
			},
			want: http.StatusSeeOther,
		},
		{
			name:   "no search text",
			method: http.MethodGet,
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				ctx = handlers.SearchParamsToCtx(ctx, search)
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
			want: http.StatusNoContent,
		},
		{
			name:   "error",
			method: http.MethodGet,
			ctx: func(ctx context.Context) context.Context {
				search := models.NewSearchRequest()
				search.Text = "test"
				ctx = handlers.SearchParamsToCtx(ctx, search)
				ctx = models.UserToCtx(ctx, &models.User{})
				return ctx
			},
			itemSvc: &MoqItemService{
				RetrieveItemsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Items, models.Pagination, error) {
					return models.Items{}, models.Pagination{}, errors.New("failed")
				},
				GetTopItemCategoriesForSearchResultsFunc: func(ctx context.Context, request *models.SearchRequest) (models.Categories, error) {
					return models.Categories{}, nil
				},
			},
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := tt.ctx(t.Context())

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, "/search", nil)
			if tt.htmx {
				req.Header.Add(htmx.HeaderRequest, "true")
			}

			mgr.HandleSearchResults(tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
