/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/immanent-tech/go-base/server/forms"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel"

	"github.com/immanent-tech/foragd/models"
)

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var tracer = otel.Tracer("github.com/immanent-tech/foragd/server/handlers")

// Manager contains the common interfaces that nearly all handlers require access to.
type Manager struct {
	AppConfig   AppConfig
	SessionMgr  SessionManager
	Breadcrumbs Breadcrumbs
}

func (m *Manager) NewPageServices() pageServices {
	return pageServices{
		appCfg:      m.AppConfig,
		sessionMgr:  m.SessionMgr,
		breadcrumbs: m.Breadcrumbs,
	}
}

// getAuthenticatedUserAndSubscriptions retrieves the user and their subscriptions from the context. It returns an error
// if the user is not logged in or subscriptions are missing.
func (m *Manager) getAuthenticatedUserAndSubscriptions(req *http.Request) (*models.User, models.Subscriptions, error) {
	user := models.UserFromCtx(req.Context())
	if user == nil {
		return nil, nil, errors.New("unauthorized")
	}
	subs := models.SubscriptionsFromCtx(req.Context())
	if subs == nil {
		return nil, nil, errors.New("subscriptions not found")
	}
	return user, subs, nil
}

func parseForm[T forms.FormInput](req *http.Request) (T, error) {
	request, err := forms.DecodeForm[T](req)
	if err != nil {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Unable to parse input",
				"This might be a temporary issue, please try again.",
			),
		}
	}
	return request, nil
}

func parseMultipartForm[T forms.FormInput](req *http.Request) (T, error) {
	request, err := forms.DecodeMultiPartForm[T](req)
	if err != nil {
		return request, &models.APIError{
			InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
			StatusCode:    http.StatusInternalServerError,
			UserMessage: models.NewErrorMessage(
				"Unable to parse input",
				"This might be a temporary issue, please try again.",
			),
		}
	}
	return request, nil
}

// PostHandlerHook is a function that can be run after a handler has done its main processing. Used mainly to perform
// route-specific or other conditional logic without complicating the handler code.
type PostHandlerHook func(res http.ResponseWriter, req *http.Request) error

const listFiltersCtxKey contextKey = "listFilters"
const listCountCtxKey contextKey = "listCount"

type contextKey string

// ListFiltersToSession stores the given list filters in the session. The path is used as a suffix so that filters are
// stored per-route.
func ListFiltersToSession(ctx context.Context, session SessionManager, path string, filters models.ListFilters) {
	session.Put(ctx, string(listFiltersCtxKey)+path, filters)
}

// ListFiltersFromSession retrieves the given list filters in the session. The path is used as a suffix so that filters
// are retrieved per-route.
func ListFiltersFromSession(ctx context.Context, session SessionManager, path string) *models.ListFilters {
	filters, ok := session.Get(ctx, string(listFiltersCtxKey)+path).(models.ListFilters)
	if !ok {
		slogctx.Warn(ctx, "Could not restore filters from session.")
		return models.NewListFilters()
	}
	return &filters
}

// ListFiltersToCtx stores the given list filters in the context.
func ListFiltersToCtx(ctx context.Context, filters models.ListFilters) context.Context {
	return context.WithValue(ctx, listFiltersCtxKey, filters)
}

// ListFiltersFromCtx retrieves the given list filters in the context.
func ListFiltersFromCtx(ctx context.Context) *models.ListFilters {
	if filters, ok := ctx.Value(listFiltersCtxKey).(models.ListFilters); ok {
		return &filters
	}
	return models.NewListFilters()
}

// ListCountToSession stores the current count of objects displayed in the list in the session. The path is used as a
// suffix so that the count is stored per-route.
func ListCountToSession(ctx context.Context, session SessionManager, path string, count int) {
	session.Put(ctx, string(listCountCtxKey)+path, count)
}

// ListCountFromSession stores the current count of objects displayed in the list in the session. The path is used as a
// suffix so that the count is retrieved per-route.
func ListCountFromSession(ctx context.Context, session SessionManager, path string) int {
	count, ok := session.Get(ctx, string(listCountCtxKey)+path).(int)
	if !ok {
		return 9
	}
	return count
}

const searchParamsCtxKey contextKey = "searchParams"
const searchCountCtxKey contextKey = "searchCount"

// SearchParamsToSession stores the given search params in the session.
func SearchParamsToSession(ctx context.Context, session SessionManager, search *models.SearchRequest) {
	session.Put(ctx, string(searchParamsCtxKey), *search)
}

// SearchParamsFromSession retrieves the given search params in the session.
func SearchParamsFromSession(ctx context.Context, session SessionManager) *models.SearchRequest {
	search, ok := session.Get(ctx, string(searchParamsCtxKey)).(models.SearchRequest)
	if !ok {
		slogctx.Warn(ctx, "Unable to restore search params from session. Using defaults.")
		return models.NewSearchRequest()
	}
	return &search
}

// SearchCountToSession stores the current count of objects displayed in the search in the session.
func SearchCountToSession(ctx context.Context, session SessionManager, count int) {
	session.Put(ctx, string(searchCountCtxKey), count)
}

// SearchCountFromSession stores the current count of objects displayed in the search in the session.
func SearchCountFromSession(ctx context.Context, session SessionManager) int {
	count, ok := session.Get(ctx, string(searchCountCtxKey)).(int)
	if !ok {
		slogctx.Warn(ctx, "Unable to restore list search count from session. Using default.")
		return 9
	}
	return count
}

// SearchParamsToCtx stores the given search params in the context.
func SearchParamsToCtx(ctx context.Context, search *models.SearchRequest) context.Context {
	return context.WithValue(ctx, searchParamsCtxKey, *search)
}

// SearchParamsFromCtx retrieves the given search params in the context.
func SearchParamsFromCtx(ctx context.Context) *models.SearchRequest {
	if search, ok := ctx.Value(searchParamsCtxKey).(models.SearchRequest); ok {
		return &search
	}
	slogctx.Warn(ctx, "No search params in context. Returning new search params.")
	return models.NewSearchRequest()
}
