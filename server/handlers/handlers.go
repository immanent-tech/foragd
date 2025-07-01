// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
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

	htmxRespCtxKey contextKey = "htmxResponse"
	titleCtxKey    contextKey = "title"
	drawerCtxKey   contextKey = "drawer"
	pageCtxKey     contextKey = "page"

	respCtxKey contextKey = "response"
)

type contextKey string

// AuthAPI represents the API surface for interacting with the auth backend.
type AuthAPI interface {
	GetAuthURL(req *http.Request) (string, error)
	CompleteUserAuth(res http.ResponseWriter, req *http.Request) error
	GetUserID(ctx context.Context) models.UserID
}

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

// RenderPage will render a full page (i.e. non-HTMX) response.
func RenderPage() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			// Get main content.
			page, ok := req.Context().Value(pageCtxKey).(templ.Component)
			if !ok {
				slogctx.FromCtx(req.Context()).Warn("Invalid or missing page content.")
				http.Error(res, "Invalid page content.", http.StatusInternalServerError)
				return
			}
			if err := page.Render(req.Context(), res); err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
			}
		})
}

// RenderContentPage will render a full page (i.e. non-HTMX) response.
func RenderContentPage() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			var drawerContent templ.Component
			// var drawerSide templ.Component
			var page templ.Component
			// Get drawer main content.
			if content := getContentFromCtx(req.Context()); len(content) != 0 {
				// Wrap main content.
				drawerContent = templ.Join(views.Header(), partials.Content(templ.Join(content...)), partials.Footer())
			}
			// // Get drawer side content.
			// if content := getDrawerSideContentFromCtx(req.Context()); len(content) != 0 {
			// 	drawerSide = partials.DrawerMenu(
			// 		partials.MenuItemTitle("Navigation"),
			// 		views.DrawerHomeLink(),
			// 		views.DrawerSubscriptionList(templ.Join(content...)),
			// 	)
			// }
			// Get page title.
			title, ok := req.Context().Value(titleCtxKey).(string)
			if !ok {
				title = "Go Feed Me"
			}
			// Render page.
			// if drawerSide != nil {
			page = templates.NewPage(title,
				partials.Drawer(drawerContent),
			).Show()
			// } else {
			// 	page = templates.NewPage(title,
			// 		drawerContent,
			// 	).Show()
			// }
			if err := page.Render(req.Context(), res); err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page content.", http.StatusInternalServerError)
			}
		})
}

// RenderContentPartials will render individual content updates (i.e., HTMX response).
func RenderContentPartials() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			var partials []templ.Component
			// Add any content updates.
			content := getContentFromCtx(req.Context())
			if len(content) > 0 {
				partials = append(partials, content...)
			} else {
				return
			}
			// // Add any drawer side-bar updates.
			// if content, ok := req.Context().Value(drawerCtxKey).(templ.Component); ok {
			// 	partials = append(partials, content)
			// }
			// Add any page title updates.
			if title, ok := req.Context().Value(titleCtxKey).(string); ok {
				partials = append(partials, templates.SetPageTitle(title))
			}
			// Get any existing htmx response writer.
			resp, ok := req.Context().Value(htmxRespCtxKey).(htmx.Response)
			if !ok {
				resp = htmx.NewResponse()
			}
			// Render as a template.
			if err := resp.RenderTempl(req.Context(), res, templ.Join(partials...)); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Template failed to render.", slog.Any("error", err))
			}

			errResp, found := req.Context().Value(respCtxKey).(*models.Response)
			if found {
				slogctx.FromCtx(req.Context()).Error("Response error.",
					slog.Any("error", errResp.Error),
				)
				res.WriteHeader(errResp.StatusCode)
			}
		})
}

// ProcessResponse handles appropriate display and logging of a models.Response object.
func ProcessResponse(res http.ResponseWriter, req *http.Request, resp *models.Response) {
	slogctx.FromCtx(req.Context()).Error("Backend returned an error.",
		slog.String("error", resp.String()),
	)
	// Write the status code.
	res.WriteHeader(resp.StatusCode)
}
