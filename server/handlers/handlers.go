// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/session"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

var (
	// ErrInvalidUser indicates the user data is invalid. This might be the case if the retrieved session is corrupted.
	ErrInvalidUser = errors.New("user data is invalid")
	// ErrMissingRequestData indicates data was expected in the request (usually in the context) but was not found.
	ErrMissingRequestData = errors.New("request data is missing")
)

var BaseChain = alice.New(
	RouteLogger,
)

// Keys for objects stored within the context and passed between handlers.
const (
	subscriptionRequestsCtxKey contextKey = "subscriptionRequests"
	subscriptionResultsCtxKey  contextKey = "subscriptionsResults"
	feedsCtxKey                contextKey = "feeds"
	subscriptionsCtxKey        contextKey = "subscriptions"

	titleCtxKey  contextKey = "title"
	drawerCtxKey contextKey = "drawer"
	pageCtxKey   contextKey = "page"

	respCtxKey contextKey = "response"
)

type contextKey string

// RouteLogger decorates the logger in the request context with routing information.
func RouteLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := slogctx.With(req.Context(),
			slog.String("route", chi.RouteContext(req.Context()).RoutePattern()),
			slog.String("method", req.Method),
		)
		ctx = slogctx.With(ctx, slog.Group("req", slog.String("id", middleware.GetReqID(ctx))))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func RenderTemplate() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the template from the context.
		template := templateFromCtx(req.Context())
		if htmx.IsHTMX(req) {
			// Get any existing htmx response writer.
			resp := htmxRespFromCtx(req.Context())
			if template != nil {
				if err := resp.RenderTempl(req.Context(), res, template); err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
					http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
					return
				}
			} else {
				if err := resp.Write(res); err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
					http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
					return
				}
				res.WriteHeader(http.StatusOK)
			}
			// Update the page title.
			if title := pageTitleFromCtx(req.Context()); title != "" {
				if err := resp.RenderTempl(req.Context(), res, templates.SetPageTitle(title)); err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to update page title.", slog.Any("error", err))
				}
			}
		} else {
			if template != nil {
				if err := template.Render(req.Context(), res); err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
					http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
					return
				}
			} else {
				res.WriteHeader(http.StatusOK)
			}
		}
	})
}

func RenderTemplateFragments(fragments ...string) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !htmx.IsHTMX(req) {
			slogctx.FromCtx(req.Context()).Error("Render template fragments called with non-htmx request.")
			http.Error(res, "Render template fragments called with non-htmx request.", http.StatusInternalServerError)
			return
		}
		// Get any existing htmx response writer.
		resp := htmxRespFromCtx(req.Context())
		// Set the fragments to be rendered.
		req = req.WithContext(partials.WithFragment(req.Context(), res, fragments...))
		err := resp.Write(res)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
			http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
			return
		}
		// Retrieve the template from the context.
		template := templateFromCtx(req.Context())
		if template != nil {
			err = template.Render(req.Context(), io.Discard)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
				return
			}
		} else {
			res.WriteHeader(http.StatusOK)
		}
		// Update the page title.
		if title := pageTitleFromCtx(req.Context()); title != "" {
			if err := resp.RenderTempl(req.Context(), res, templates.SetPageTitle(title)); err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to update page title.", slog.Any("error", err))
			}
		}
	})
}

// TriggerStateUpdates adds a htmx trigger to the response to send the "updateState" event, which elements on a page may
// listen to for updating their state.
func TriggerStateUpdates(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		slogctx.FromCtx(req.Context()).Debug("Adding updateState event trigger.")
		resp := htmxRespFromCtx(req.Context())
		ctx := htmxRespToCtx(req.Context(), resp.AddTrigger(htmx.Trigger("updateState")))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// SetupRedirect handler will add a HX-Location header to the request when the given path is non-nil and the request has
// been made through HTMX.
func SetupRedirect(path *string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			if htmx.IsHTMX(req) && path != nil {
				var route string
				var values map[string]string
				switch {
				case path == nil:
					route = "/home"
				case *path == "/subscriptions":
					route = *path
					values = session.SubscriptionFiltersFromSession(ctx).Parameters()
				case *path == "/articles":
					route = *path
					values = session.ArticleFiltersFromSession(ctx).Parameters()
				}
				// Set-up client-side redirect to view.
				htmxResp := htmx.NewResponse().LocationWithContext(
					route,
					htmx.LocationContext{
						Target: partials.ContentID.Target(),
						Values: values,
					})
				ctx = context.WithValue(ctx, htmxRespCtxKey, htmxResp)
				slogctx.FromCtx(ctx).Debug("Redirect in place.",
					slog.String("redirect", route),
				)
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SavePageState saves the current page state in the session.
func SavePageState(filters any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Generate state.
			session.FiltersToSession(req.Context(), filters)
			next.ServeHTTP(res, req)
		})
	}
}

func RenderError(res http.ResponseWriter, req *http.Request, resp *models.Response) {
	slogctx.FromCtx(req.Context()).Error("Backend returned an error.",
		slog.String("error", resp.String()),
	)
	res.WriteHeader(resp.StatusCode)
	res.Write([]byte(resp.String()))
	// // Write the status code.
	// res.WriteHeader(resp.StatusCode)
}

// ProcessResponse handles appropriate display and logging of a models.Response object.
func ProcessResponse(res http.ResponseWriter, req *http.Request, resp *models.Response) {
	slogctx.FromCtx(req.Context()).Error("Backend returned an error.",
		slog.String("error", resp.String()),
	)
	ctx := templateToCtx(req.Context(), partials.ShowNotification(
		&models.UserMessage{
			Status:  models.UserMessageStatusWarning,
			Summary: "A backend error occurred processing the action.",
		},
	))
	RenderTemplate().ServeHTTP(res, req.WithContext(ctx))
	// Write the status code.
	res.WriteHeader(resp.StatusCode)
}
