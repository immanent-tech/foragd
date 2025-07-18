// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

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

// ErrNoCtxData indicates that required data could not be retrieved from context values.
var ErrNoCtxData = errors.New("missing data in context")

var BaseChain = alice.New(
	RouteLogger,
)

// Keys for objects stored within the context and passed between handlers.
const (
	titleCtxKey contextKey = "title"
	respCtxKey  contextKey = "response"
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

// RenderResponse handles rendering a response. If the response contains a template, that will be rendered in the http
// response. If the response contains an error, it will be logged.
func RenderResponse(resp *models.Response) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get any existing htmx response writer.
		htmxResp := htmxRespFromCtx(req.Context())
		if resp == nil {
			// If there is no response, return 200: OK.
			res.WriteHeader(http.StatusOK)
			err := htmxResp.Write(res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Problem writing response.",
					slog.Any("error", err),
				)
			}
			return
		}
		// If the response contains an error, log it.
		if resp.InternalError != nil {
			switch {
			case resp.StatusCode < 400:
				slogctx.FromCtx(req.Context()).Debug(resp.InternalError.Error())
			case resp.StatusCode < 500:
				slogctx.FromCtx(req.Context()).Warn(resp.InternalError.Error())
			default:
				slogctx.FromCtx(req.Context()).Error(resp.InternalError.Error())
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
			// Update the page title.
			if title := pageTitleFromCtx(req.Context()); title != "" {
				err := htmxResp.RenderTempl(req.Context(), res, templates.SetPageTitle(title))
				if err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to update page title.", slog.Any("error", err))
				}
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

// TriggerEvents will add event triggers for the list of given events to the response.
func TriggerEvents(events ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			slogctx.FromCtx(req.Context()).Debug("Triggering event listeners.",
				slog.String("events", strings.Join(events, ",")),
			)
			resp := htmxRespFromCtx(req.Context())
			for event := range slices.Values(events) {
				resp = resp.AddTrigger(htmx.Trigger(event))
			}
			ctx := htmxRespToCtx(req.Context(), resp)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
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

func RespInvalidInput(err error) *models.Response {
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusUnprocessableEntity),
		models.WithResponseError(err),
		models.WithResponseTemplate(
			partials.Alert(partials.MsgBadInput()),
		),
	)
}

func RespBackendError(err error) *models.Response {
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusInternalServerError),
		models.WithResponseError(err),
		models.WithResponseTemplate(
			partials.Alert(partials.MsgBackendErr()),
		),
	)
}

func RespForbidden() *models.Response {
	return models.NewResponse(
		models.WithResponseStatusCode(http.StatusForbidden),
		models.WithResponseTemplate(
			partials.Alert(partials.MsgBadInput()),
		),
	)
}

func NotFound() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		resp := models.NewResponse(
			models.WithResponseTemplate(templates.NotFound()),
			models.WithResponseStatusCode(http.StatusNotFound),
		)
		alice.New(
			RouteLogger,
		).Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}
