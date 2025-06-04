// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/components/session"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// SaveFilters extracts the filters from the request params and stores them in the context.
func SaveFilters(params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Retrieve filters.
			var (
				filters *models.Filters
				err     error
			)
			filters, err = models.NewFiltersFromParams(params)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to extract filters from params.",
					slog.Any("error", err),
				)
				filters = models.NewFilters()
			}
			ctx := models.FiltersToCtx(req.Context(), *filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SaveState saves the current page state in the session.
func SetResponseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Vary", "HX-Request")
		next.ServeHTTP(res, req)
	})
}

// SetupRedirect handler will add a HX-Location header to the request when the given path is non-nil and the request has
// been made through HTMX.
func SetupRedirect(path *string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if htmx.IsHTMX(req) && path != nil {
				slogctx.FromCtx(req.Context()).Debug("Setting-up client-side redirect.",
					slog.String("path", *path),
				)
				view := RestorePageState(req.Context(), *path)
				// view := models.GetPreviousViewedPage(req.Context(), api)
				HxLocationData := HXLocation{Path: view.String(), Target: partials.ContentID.Target()}
				data, err := json.Marshal(HxLocationData)
				if err != nil {
					ProcessResponse(res, req, &models.Response{
						StatusCode:    http.StatusInternalServerError,
						InternalError: err,
						UserMessage: &models.UserMessage{
							Status:  models.UserMessageStatusError,
							Summary: "Redirection failed.",
						},
					})
				}
				// Set-up client-side redirect to view.
				res.Header().Add(htmx.HeaderLocation, string(data))
			}
			next.ServeHTTP(res, req)
		})
	}
}

// SavePageState saves the current page state in the session.
func SavePageState(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Save page state.
		state := models.PageState{Path: req.URL.Path, Params: req.URL.Query()}
		ctx := models.PageStateToCtx(req.Context(), state)
		slogctx.FromCtx(ctx).Debug("Saved page state to context.",
			slog.String("state", state.String()),
		)
		// Store page states for some paths into session for history restoration.
		if req.Method == http.MethodGet {
			switch req.URL.Path {
			case "/home/subscriptions":
				session.Manager.Put(req.Context(), subscriptionsPageState, state)
			case "/home/articles":
				session.Manager.Put(req.Context(), articlesPageState, state)
			}
			slogctx.FromCtx(ctx).Debug("Saved page state to session.",
				slog.String("state", state.String()),
			)
		}
		// Pass control to next handler.
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func RestorePageState(ctx context.Context, path string) models.PageState {
	switch path {
	case "/home/subscriptions":
		if state, ok := session.Manager.Get(ctx, subscriptionsPageState).(models.PageState); ok {
			return state
		}
		return models.PageState{Path: "/home/subscriptions", Params: models.NewFilters().ToQueryParams()}
	case "/home/articles":
		if state, ok := session.Manager.Get(ctx, articlesPageState).(models.PageState); ok {
			return state
		}
		return models.PageState{Path: "/home/articles", Params: models.NewFilters().ToQueryParams()}
	default:
		return models.PageState{Path: "/home"}
	}
}
