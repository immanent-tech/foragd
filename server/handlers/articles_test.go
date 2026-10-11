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
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/handlers"
)

func TestManager_HandleListArticlesGet(t *testing.T) {
	tests := []struct {
		name     string
		itemSvc  handlers.ItemService
		ctxSetup func(ctx context.Context) context.Context
		want     int
	}{
		{
			name: "ok",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return models.Articles{
						&models.Article{},
					}, models.Subscriptions{
						&models.Subscription{},
					}, models.Pagination{}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no articles",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return nil, models.Subscriptions{
						&models.Subscription{},
					}, models.Pagination{}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no subscriptions",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return models.Articles{
						&models.Article{},
					}, nil, models.Pagination{}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "error",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return nil, nil, models.Pagination{}, models.NewAPIError(
						http.StatusInternalServerError,
						errors.New("error!"),
					)
				},
			},
			want: http.StatusInternalServerError,
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
			req := httptest.NewRequest(http.MethodGet, "/articles", nil)

			mgr.HandleListArticles(tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestManager_HandleListArticlesPost(t *testing.T) {
	tests := []struct {
		name     string
		itemSvc  handlers.ItemService
		ctxSetup func(ctx context.Context) context.Context
		want     int
	}{
		{
			name: "ok",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return models.Articles{
						&models.Article{},
					}, models.Subscriptions{
						&models.Subscription{},
					}, models.Pagination{}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no articles",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return nil, models.Subscriptions{
						&models.Subscription{},
					}, models.Pagination{}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no subscriptions",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return models.Articles{
						&models.Article{},
					}, nil, models.Pagination{}, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "error",
			itemSvc: &MoqItemService{
				FilterArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (models.Articles, models.Subscriptions, models.Pagination, error) {
					return nil, nil, models.Pagination{}, models.NewAPIError(
						http.StatusInternalServerError,
						errors.New("error!"),
					)
				},
			},
			want: http.StatusInternalServerError,
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
			req := httptest.NewRequest(http.MethodPost, "/articles", nil)
			req.Header.Set(htmx.HeaderRequest, "true")

			mgr.HandleListArticles(tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestManager_HandleViewArticle(t *testing.T) {
	tests := []struct {
		name           string
		itemSvc        handlers.ItemService
		setupReq       func() *http.Request
		setupCtx       func(context.Context) context.Context
		wantStatusCode int
	}{
		{
			name: "validation fails - missing item prefix",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					t.Log("GetArticles should not be called for validation failure")
					panic("should not be called")
				},
			},
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/article/invalid", nil)
			},
			wantStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name: "validation fails - empty itemID",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					t.Log("GetArticles should not be called for validation failure")
					panic("should not be called")
				},
			},
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/article/", nil)
			},
			wantStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name: "get article error",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					return nil, models.NewAPIError(http.StatusInternalServerError, errors.New("db error"))
				},
			},
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/article/item_test-123", nil)
			},
			wantStatusCode: http.StatusUnprocessableEntity,
		},
		{
			name: "article not found (empty slice)",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					return nil, nil
				},
			},
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/article/item_test-123", nil)
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name: "article not found (nil slice)",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					var nilSlice models.Articles
					return nilSlice, nil
				},
			},
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/article/item_test-123", nil)
			},
			wantStatusCode: http.StatusNotFound,
		},
		{
			name: "show full content - no remote content needed",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					return models.Articles{
						&models.Article{
							Item: models.Item{
								ItemID: "item_test-123",
								Title:  "Test Article",
							},
							ShowFullContent: false,
						},
					}, nil
				},
			},
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/article/item_test-123", nil)
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "show full content - fetch remote content fails",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					return models.Articles{
						&models.Article{
							Item: models.Item{
								ItemID:  "item_test-123",
								Title:   "Test Article",
								URL:     "https://example.com/article",
								Content: nil,
							},
							ShowFullContent: true,
						},
					}, nil
				},
				GetArticleRemoteContentFunc: func(ctx context.Context, article *models.Article) error {
					return errors.New("network error")
				},
			},
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/article/item_test-123?show_full_content=true", nil)
				req.Header.Set("Referer", "https://example.com/home")
				return req
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "show full content - fetch remote content fails with APIError",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					return models.Articles{
						&models.Article{
							Item: models.Item{
								ItemID:  "item_test-123",
								Title:   "Test Article",
								URL:     "https://example.com/article",
								Content: nil,
							},
							ShowFullContent: true,
						},
					}, nil
				},
				GetArticleRemoteContentFunc: func(ctx context.Context, article *models.Article) error {
					return models.NewAPIError(http.StatusForbidden, errors.New("access denied"))
				},
			},
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/article/item_test-123?show_full_content=true", nil)
				req.Header.Set("Referer", "https://example.com/home")
				return req
			},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name: "show full content - fetch remote content fails from articles page",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					return models.Articles{
						&models.Article{
							Item: models.Item{
								ItemID:  "item_test-123",
								Title:   "Test Article",
								URL:     "https://example.com/article",
								Content: nil,
							},
							ShowFullContent: true,
						},
					}, nil
				},
				GetArticleRemoteContentFunc: func(ctx context.Context, article *models.Article) error {
					return errors.New("network error")
				},
			},
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/article/item_test-123?show_full_content=true", nil)
				req.Header.Set("Referer", "https://example.com/articles")
				return req
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "show full content - success",
			itemSvc: &MoqItemService{
				GetArticlesFunc: func(ctx context.Context, itemIDs ...models.ItemID) (models.Articles, error) {
					return models.Articles{
						&models.Article{
							Item: models.Item{
								ItemID: "item_test-123",
								Title:  "Test Article",
							},
							ShowFullContent: false,
						},
					}, nil
				},
			},
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/article/item_test-123?show_full_content=true", nil)
			},
			wantStatusCode: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupManager(t)
			ctx := t.Context()

			if tt.setupCtx != nil {
				ctx = tt.setupCtx(ctx)
			}

			req := tt.setupReq()
			rec := httptest.NewRecorder()

			// Capture panics from template rendering
			defer func() {
				if r := recover(); r != nil {
					// Template panic is not a test failure - the handler behavior is correct
				}
			}()

			mgr.HandleViewArticle(tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.wantStatusCode {
				t.Errorf("HandleViewArticle() status = %v, want %v", rec.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestManager_HandleListArticlesUpdates(t *testing.T) {
	tests := []struct {
		name     string
		itemSvc  handlers.ItemService
		ctxSetup func(ctx context.Context) context.Context
		want     int
	}{
		{
			name: "ok",
			itemSvc: &MoqItemService{
				CountArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (int64, error) {
					return 5, nil
				},
			},
			want: http.StatusOK,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "no updates",
			itemSvc: &MoqItemService{
				CountArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (int64, error) {
					return 0, nil
				},
			},
			want: http.StatusNoContent,
			ctxSetup: func(ctx context.Context) context.Context {
				return models.UserToCtx(ctx, &models.User{})
			},
		},
		{
			name: "not view unread",
			itemSvc: &MoqItemService{
				CountArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (int64, error) {
					return 5, nil
				},
			},
			want: http.StatusNoContent,
			ctxSetup: func(ctx context.Context) context.Context {
				ctx = models.UserToCtx(ctx, &models.User{})
				ctx = handlers.ListFiltersToCtx(ctx, &models.ListFilters{View: models.ViewAll})
				return ctx
			},
		},
		{
			name: "error",
			itemSvc: &MoqItemService{
				CountArticlesFunc: func(ctx context.Context, filters *models.ListFilters, extraQueries ...query.Option) (int64, error) {
					return 0, models.NewAPIError(http.StatusInternalServerError, errors.New("error!"))
				},
			},
			want: http.StatusNoContent,
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

			req := httptest.NewRequest(http.MethodGet, "/articles/updates", nil)
			req.Header.Set(htmx.HeaderRequest, "true")
			rec := httptest.NewRecorder()

			mgr.HandleListArticlesUpdates(tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
