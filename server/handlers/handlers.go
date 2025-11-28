// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	"github.com/russross/blackfriday/v2"
	slogchi "github.com/samber/slog-chi"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web/templates"
)

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
	// ErrBackendAPIError indicates that an invalid response was received from a backend API.
	ErrBackendAPIError = errors.New("backend API reported error")
)

var defaultHandlerChain = alice.New(
	storePath,
)

// NotFound handles showing a page for a 404 response.
func NotFound() http.HandlerFunc {
	return alice.New().Then(renderPage(templates.NotFound(), "Not Found")).ServeHTTP
}

// CSRFError handles CSRF error conditions. It will log details about the request then show an error page to the user.
func CSRFError() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		params := make(map[string]string)
		if chi.RouteContext(req.Context()) != nil {
			if len(chi.RouteContext(req.Context()).URLParams.Keys) > 0 {
				for i, k := range chi.RouteContext(req.Context()).URLParams.Keys {
					params[k] = chi.RouteContext(req.Context()).URLParams.Values[i]
				}
			}
		}
		slogctx.FromCtx(req.Context()).Error("CSRF check failed",
			slog.String("method", req.Method),
			slog.String("host", req.Host),
			slog.String("path", req.URL.Path),
			slog.String("query", req.URL.RawQuery),
			slog.Any("params", params),
			slog.String("route", chi.RouteContext(req.Context()).RoutePattern()),
			slog.String("ip", req.RemoteAddr),
			slog.String("referer", req.Referer()),
			slog.String(slogchi.RequestIDKey, middleware.GetReqID(req.Context())),
		)
		renderPage(templates.ErrorPage(
			models.NewErrorMessage("CSRF Check Failed", "Cannot complete your request."),
		), "Error")
		res.WriteHeader(http.StatusBadRequest)
	}).ServeHTTP
}

// StaticFileHandler handles serving content from the embedded filesystem containing static assets (i.e., images,
// etc.).
func StaticFileHandler(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Check, if the requested file is existing.
		_, err := fs.Open(req.URL.Path)
		if err != nil {
			// If file is not found, return HTTP 404 error.
			http.NotFound(res, req)
			return
		}
		// File is found, return to standard http.FileServer.
		http.FileServer(fs).ServeHTTP(res, req)
	})
}

// DocsHandler handles serving markdown documents from the docs content directory.
func DocsHandler(fs embed.FS) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		doc := chi.URLParam(req, "*")
		// Check, if the requested file is existing.
		contents, err := fs.ReadFile("content/docs/" + doc + ".md")
		if err != nil {
			// If file is not found, return HTTP 404 error.
			http.NotFound(res, req)
			return fmt.Errorf("unable to render document %s: %w", doc, err)
		}
		output := blackfriday.Run(contents, blackfriday.WithExtensions(blackfriday.AutoHeadingIDs))
		template := templates.Page(strings.ToTitle(doc)+" - "+config.AppName, templates.Document(output))
		err = template.Render(req.Context(), res)
		if err != nil {
			return fmt.Errorf("unable to render document %s: %w", doc, err)
		}
		return nil
	})).ServeHTTP
}

func handlerWithError(f func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		err := f(res, req)
		if err != nil {
			var apiErr interface {
				error
				HTTPStatus() int
			}
			if errors.As(err, &apiErr) {
				switch {
				case apiErr.HTTPStatus() < 400: //nolint:mnd // easier to read as a number.
					slogctx.FromCtx(req.Context()).DebugContext(req.Context(), apiErr.Error())
				case apiErr.HTTPStatus() < 500: //nolint:mnd // easier to read as a number.
					slogctx.FromCtx(req.Context()).WarnContext(req.Context(), apiErr.Error())
				default:
					slogctx.FromCtx(req.Context()).ErrorContext(req.Context(), apiErr.Error())
				}
				res.WriteHeader(apiErr.HTTPStatus())
			} else {
				slogctx.FromCtx(req.Context()).ErrorContext(req.Context(),
					"Unknown API Error.",
					slog.Any("error", err),
				)
				http.Error(res, err.Error(), http.StatusInternalServerError)
			}
		}
	}
}

// HXLocationRequest defines the value of the HX-Location header.
//
// https://htmx.org/headers/hx-location/
type HXLocationRequest struct {
	// The URL path.
	Path string `json:"path"`
	//  The source element of the request.
	Source string `json:"source,omitzero"`
	// An event that “triggered” the request.
	Event string `json:"event,omitzero"`
	// A JS callback that will handle the response HTML.
	Handler string `json:"handler,omitzero"`
	// The target to swap the response into.
	Target string `json:"target,omitzero"`
	// How the response will be swapped in relative to the target.
	Swap string `json:"swap,omitzero"`
	// Values to submit with the request.
	Values map[string]any `json:"values,omitzero"`
	// Headers to submit with the request.
	Headers map[string]string `json:"headers,omitzero"`
	// Allows you to select the content you want swapped from a response.
	Select string `json:"select,omitzero"`
	// Set to 'false' or a path string to prevent or override the URL pushed to browser location history
	Push string `json:"push,omitzero"`
	// A path string to replace the URL in the browser location history
	Replace string `json:"replace,omitzero"`
}

// SetRedirect adds the HX-Location header with the given values to the response, which triggers a client side
// redirection without reloading the whole page.
//
// https://htmx.org/headers/hx-location/
func SetRedirect(res http.ResponseWriter, request HXLocationRequest) error {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("set redirect: marshal request: %w", err)
	}
	res.Header().Set(htmx.HeaderLocation, string(requestJSON))
	return nil
}

// renderPage will render the given template as a full page. It handles htmx and non-htmx requests, rendering the
// appropriate full or partial HTML response as appropriate.
func renderPage(template templ.Component, title string) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if template == nil {
			// If there is no response, return 204: No Content.
			res.WriteHeader(http.StatusNoContent)
			return
		}
		if !IsHTMX(req) || IsHistoryRestoreRequest(req) { // Non-HTMX or HistoryRestoreRequests render a full-page.
			user := models.UserFromCtx(req.Context())
			if user == nil {
				templ.Handler(templates.Page(title, templates.ErrorPage(
					models.NewErrorMessage("Invalid request", "This might be a temporary error, please try again."),
				))).ServeHTTP(res, req)
				return
			}
			template = templates.Content(user, template)
			templ.Handler(templates.Page(title, template)).ServeHTTP(res, req)
			return
		}
		// HTMX request renders partial content.
		// Add OOB swaps depending on path.
		template = templ.Join(template,
			templates.SideBar(templ.Attributes{"hx-swap-oob": "true"}),
			templates.Dock(templ.Attributes{"hx-swap-oob": "true"}),
		)
		// Update page title if set.
		if title != "" {
			// Update the page title if set.
			template = templ.Join(template, templates.SetPageTitle(title))
		}
		// Add OOB swap to update CSRF token.
		template = templ.Join(template, templates.UpdateCSRFToken())
		// Render template (or template fragment).
		target := templates.FragmentKey(req.Header.Get(htmx.HeaderTarget))
		if target != "" && target != templates.FragmentContent {
			templ.Handler(template, templ.WithFragments(target)).ServeHTTP(res, req)
		} else {
			templ.Handler(template).ServeHTTP(res, req)
		}
	})
}

// renderPartial will render the given template, optionally updating the page title if one is given.
func renderPartial(template templ.Component) http.Handler {
	return templ.Handler(templ.Join(template, templates.UpdateCSRFToken()))
}

// IsHTMX returns a boolean indicating whether the request is a HTMX request.
func IsHTMX(req *http.Request) bool {
	return req.Header.Get("HX-Request") == "true" //nolint:goconst // unnecessary.
}

// IsHistoryRestoreRequest returns a boolean indicating whether the request is a HTMX history restore request.
func IsHistoryRestoreRequest(req *http.Request) bool {
	return req.Header.Get("HX-History-Restore-Request") == "true"
}

func parseFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		filters, valid, err := forms.DecodeForm[*models.ListDisplayFilters](req)
		ctx := req.Context()
		switch {
		case err != nil:
			if errors.Is(err, forms.ErrNoFormData) {
				restored := session.RestoreFromSession(ctx, "filters_"+req.URL.Path, models.NewListDisplayFilters)
				filters = &restored
				ctx = models.PageFiltersToCtx(req.Context(), req.URL.Path, filters)
				slogctx.FromCtx(ctx).Debug("No form data. Using filters from session.",
					slog.String("filters", filters.QueryString()))
			} else {
				slogctx.FromCtx(ctx).Debug("Error parsing filters. Using default filters.")
				newFilters := models.NewListDisplayFilters()
				session.SaveToSession(ctx, "filters_"+req.URL.Path, newFilters)
				ctx = models.PageFiltersToCtx(req.Context(), req.URL.Path, filters)
			}
		case !valid:
			slogctx.FromCtx(ctx).Debug("Invalid filters. Using default.")
			newFilters := models.NewListDisplayFilters()
			session.SaveToSession(ctx, "filters_"+req.URL.Path, newFilters)
			ctx = models.PageFiltersToCtx(req.Context(), req.URL.Path, filters)
		default:
			slogctx.FromCtx(ctx).Debug("Saving filters.",
				slog.String("filters", filters.QueryString()))
			session.SaveToSession(ctx, "filters_"+req.URL.Path, *filters)
			ctx = models.PageFiltersToCtx(req.Context(), req.URL.Path, filters)
		}
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func storePath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := models.PathToCtx(req.Context(), req.URL.Path)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func setCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Cache-Control", "max-age=60, must-revalidate")
		next.ServeHTTP(res, req)
	})
}

//nolint:gocognit
func watchForUpdates(api *elastic.API, watch query.Option) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			res.WriteHeader(http.StatusNoContent)
			slogctx.FromCtx(req.Context()).Error("Unable to watch for updates.",
				slog.Any("error", models.ErrNoUserCtx),
			)
			return
		}
		updateInterval, err := time.ParseDuration(user.GetSettings().UpdatesFrequency)
		if err != nil {
			res.WriteHeader(http.StatusNoContent)
			slogctx.FromCtx(req.Context()).Error("Unable to watch for updates.",
				slog.Any("error", err),
			)
			return
		}

		// Set headers for SSE.
		res.Header().Set("Content-Type", "text/event-stream")
		res.Header().Set("Cache-Control", "no-cache")
		res.Header().Set("Connection", "keep-alive")
		res.Header().Set("X-Accel-Buffering", "no")
		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		} else {
			slogctx.FromCtx(req.Context()).Warn("Cannot flush update stream!")
			res.WriteHeader(http.StatusNoContent)
		}
		var (
			currentCount int64
			prevCount    int64
		)
		prevCount, err = api.CountItems(req.Context(), watch)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot get updates count.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		for {
			select {
			case <-req.Context().Done():
				res.Header().Set("Connection", "close")
				res.WriteHeader(http.StatusRequestTimeout)
				return
			default:
				currentCount, err = api.CountItems(req.Context(), watch)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					slogctx.FromCtx(req.Context()).Debug("Subscription updates found.")
					var respBuf bytes.Buffer
					template := bufio.NewWriter(&respBuf)
					err := templates.UpdatesToast().Render(req.Context(), template)
					if err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to render template.",
							slog.Any("error", err))
						continue
					}
					err = template.Flush()
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to flush SSE message buffer.",
							slog.Any("error", err))
					}
					_, err = fmt.Fprintf(res, "data: %s\n\n", respBuf.String())
					if err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to send update SSE message.",
							slog.Any("error", err))
					}
					if f, ok := res.(http.Flusher); ok {
						f.Flush()
					}
				}
				prevCount = currentCount
				time.Sleep(updateInterval)
			}
		}
	})
}
