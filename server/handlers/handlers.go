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

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/server/session"
	"github.com/immanent-tech/go-feed-me/web/templates"
	"github.com/immanent-tech/go-feed-me/web/templates/layouts"
	"github.com/immanent-tech/go-feed-me/web/templates/pages"
	"github.com/immanent-tech/go-feed-me/web/templates/partials"
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

func logError(err error) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var apiErr interface {
				error
				HTTPStatus() int
			}
			if errors.As(err, &apiErr) {
				switch {
				case apiErr.HTTPStatus() < 400:
					slogctx.FromCtx(req.Context()).DebugContext(req.Context(), apiErr.Error())
				case apiErr.HTTPStatus() < 500:
					slogctx.FromCtx(req.Context()).WarnContext(req.Context(), apiErr.Error())
				default:
					slogctx.FromCtx(req.Context()).ErrorContext(req.Context(), apiErr.Error())
				}
			}
		})
	}
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
		if htmx.IsHTMX(req) {
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

// render handles rendering a response. If the response contains a template, that will be rendered in the http
// response. If the response contains an error, it will be logged.
func renderTemplate(template templ.Component, title string) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get any existing htmx response writer.
		htmxResp := htmxRespFromCtx(req.Context())
		if template == nil {
			// If there is no response, return 204: No Content.
			res.WriteHeader(http.StatusNoContent)
			err := htmxResp.Write(res)
			if err != nil {
				slogctx.FromCtx(req.Context()).ErrorContext(req.Context(), "Problem writing response.",
					slog.Any("error", err),
				)
			}
			return
		}
		// Write the response template.
		if htmx.IsHTMX(req) {
			if htmx.IsHistoryRestoreRequest(req) {
				template = templates.Page(title, layouts.Drawer(template))
			} else if title != "" {
				// Update the page title if set.
				template = templ.Join(template, templates.SetPageTitle(title))
			}
			err := htmxResp.RenderTempl(req.Context(), res, template)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
		} else {
			template = templates.Page(title, layouts.Drawer(template))
			err := template.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
		}
	})
}

func handlerWithError(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		err := f(res, req)
		if err != nil {
			logError(err)
			http.Error(res, err.Error(), models.HTTPStatus(err))
		}
	}
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
