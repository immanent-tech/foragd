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
			req := httptest.NewRequest(http.MethodGet, "/list/articles", nil)

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
			req := httptest.NewRequest(http.MethodPost, "/list/articles", nil)
			req.Header.Set(htmx.HeaderRequest, "true")

			mgr.HandleListArticles(tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
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

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/list/articles/updates", nil)
			req.Header.Set(htmx.HeaderRequest, "true")

			mgr.HandleListArticlesUpdates(tt.itemSvc)(rec, req.WithContext(ctx))

			if rec.Code != tt.want {
				t.Fatalf("got status %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
