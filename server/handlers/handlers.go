// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/session"
	"github.com/joshuar/go-feed-me/web/templates/pages"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

var (
	// ErrNoCtxData indicates that required data could not be retrieved from context values.
	ErrNoCtxData = errors.New("missing data in context")
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
)

// Keys for objects stored within the context and passed between handlers.
const (
	titleCtxKey contextKey = "title"
	// defaultUpdateInterval is the default interval for checking for updates (i.e., for update notifications).
	defaultUpdateInterval = time.Minute
)

type contextKey string

// routeLogger decorates the logger in the request context with routing information.
func routeLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := slogctx.With(req.Context(),
			slog.String("route", chi.RouteContext(req.Context()).RoutePattern()),
			slog.String("method", req.Method),
		)
		ctx = slogctx.With(ctx, slog.Group("req", slog.String("id", middleware.GetReqID(ctx))))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// render handles rendering a response. If the response contains a template, that will be rendered in the http
// response. If the response contains an error, it will be logged.
func render(resp *models.Response) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get any existing htmx response writer.
		htmxResp := htmxRespFromCtx(req.Context())
		if resp == nil {
			// If there is no response, return 200: OK.
			res.WriteHeader(http.StatusOK)
			err := htmxResp.Write(res)
			if err != nil {
				slogctx.FromCtx(req.Context()).ErrorContext(req.Context(), "Problem writing response.",
					slog.Any("error", err),
				)
			}
			return
		}
		// If the response contains an error, log it.
		if resp.InternalError != nil {
			switch {
			case resp.StatusCode < 400:
				slogctx.FromCtx(req.Context()).DebugContext(req.Context(), resp.InternalError.Error())
			case resp.StatusCode < 500:
				slogctx.FromCtx(req.Context()).WarnContext(req.Context(), resp.InternalError.Error())
			default:
				slogctx.FromCtx(req.Context()).ErrorContext(req.Context(), resp.InternalError.Error())
			}
		}
		// Write the response status code.
		res.WriteHeader(resp.StatusCode)
		// If there is no template to render, return.
		if resp.Template == nil {
			return
		}
		// Write the response template.
		if htmx.IsHTMX(req) { //nolint:nestif // TODO: can this logic be simplified?
			err := htmxResp.RenderTempl(req.Context(), res, resp)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
		} else {
			err := resp.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
		}
	})
}

// SetupRedirect handler will add a HX-Location header to the request when the given path is non-nil and the request has
// been made through HTMX.
func SetupRedirect(path string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			if htmx.IsHTMX(req) {
				var route string
				var values map[string]string
				var pushURLPath string
				switch path {
				case "/subscriptions":
					route = path
					filters := session.SubscriptionFiltersFromSession(ctx)
					values = filters.Parameters()
					pushURLPath = route + "?" + filters.Query()
				case "/articles":
					route = path
					filters := session.ArticleFiltersFromSession(ctx)
					values = filters.Parameters()
					pushURLPath = route + "?" + filters.Query()
				default:
					route = "/home"
					pushURLPath = route
				}
				// Set-up client-side redirect to view.
				htmxResp := htmx.NewResponse().LocationWithContext(
					route,
					htmx.LocationContext{
						Target: partials.ContentID.Target(),
						Values: values,
					})
				htmxResp = htmxResp.PushURL(pushURLPath)
				ctx = context.WithValue(ctx, htmxRespCtxKey, htmxResp)
				slogctx.FromCtx(ctx).Debug("Redirect in place.",
					slog.String("redirect", route),
				)
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// savePageState saves the current page state in the session.
func savePageState(filters any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Generate state.
			session.FiltersToSession(req.Context(), filters)
			next.ServeHTTP(res, req)
		})
	}
}

func RespInvalidInput(err error) *models.Response {
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusUnprocessableEntity),
		models.WithResponseError(err),
		models.WithResponseTemplate(
			partials.Notification(partials.MsgBadInput()),
		),
	)
}

func RespBackendError(err error) *models.Response {
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusInternalServerError),
		models.WithResponseError(err),
		models.WithResponseTemplate(
			partials.Notification(partials.MsgBackendErr()),
		),
	)
}

func RespForbidden() *models.Response {
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusForbidden),
		models.WithResponseTemplate(
			partials.Notification(partials.MsgBadInput()),
		),
	)
}

func NotFound() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		resp := models.NewResponse(
			models.WithResponseTemplate(pages.NotFound()),
			models.WithResponseStatusCode(http.StatusNotFound),
		)
		alice.New(
			routeLogger,
		).Then(render(resp)).ServeHTTP(res, req)
	}
}
