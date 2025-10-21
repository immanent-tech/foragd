// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	"github.com/russross/blackfriday/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web/templates"
)

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var defaultHandlerChain = alice.New(
	storePath,
)

const (
	// defaultUpdateInterval is the default interval for checking for updates (i.e., for update notifications).
	defaultUpdateInterval = time.Minute
)

// NotFound handles showing a page for a 404 response.
func NotFound() http.HandlerFunc {
	return alice.New().Then(renderPage(templates.NotFound(), "Not Found")).ServeHTTP
}

// StaticFileServerHandler handles serving content from the embedded filesystem containing static assets (i.e., images,
// etc.).
func StaticFileServerHandler(fs http.FileSystem) http.Handler {
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

func ImageProxy() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		url := chi.URLParam(req, "*")
		// image := filepath.Base(url)
		// host := filepath.Dir(url)
		resp, err := http.Get("http://imgproxy:8080/https://" + url)
		if err != nil {
			res.WriteHeader(resp.StatusCode)
			return fmt.Errorf("unable to proxy image: %w", err)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			res.WriteHeader(http.StatusInternalServerError)
			return fmt.Errorf("unable to proxy image: %w", err)
		}
		res.WriteHeader(http.StatusOK)
		res.Write(b)
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
				case apiErr.HTTPStatus() < 400:
					slogctx.FromCtx(req.Context()).DebugContext(req.Context(), apiErr.Error())
				case apiErr.HTTPStatus() < 500:
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

// SetRedirect sets headers for performing a HTMX redirect to the given path.
func SetRedirect(ctx context.Context, path string, filters models.Filters, res http.ResponseWriter) {
	pushURLPath := path
	locCtx := htmx.LocationContext{
		Target: templates.ContentID.Target(),
	}
	if filters != nil {
		locCtx.Values = filters.Values()
		pushURLPath = path + "?" + filters.QueryString()
	}
	htmxResp := htmx.NewResponse().LocationWithContext(path, locCtx).PushURL(pushURLPath)
	slogctx.FromCtx(ctx).Debug("Redirecting.",
		slog.String("path", pushURLPath),
	)
	err := htmxResp.Write(res)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to set redirect.",
			slog.String("path", path),
			slog.Any("error", err),
		)
	}
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
			template = templates.Content(template)
			templ.Handler(templates.Page(title, template)).ServeHTTP(res, req)
			return
		} else { // HTMX request renders partial content.
			// Add OOB swaps depending on path.
			template = templ.Join(template,
				templates.ContentSideBar(templ.Attributes{"hx-swap-oob": "true"}),
				templates.ContentDock(templ.Attributes{"hx-swap-oob": "true"}),
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
		}
	})
}

// renderPartial will render the given template, optionally updating the page title if one is given.
func renderPartial(template templ.Component) http.Handler {
	return templ.Handler(templ.Join(template, templates.UpdateCSRFToken()))
}

func IsHTMX(req *http.Request) bool {
	return req.Header.Get("HX-Request") == "true"
}

func IsHistoryRestoreRequest(req *http.Request) bool {
	return req.Header.Get("HX-History-Restore-Request") == "true"
}

// RenderMarkdown handles displaying a document, such as the privacy policy or terms of service..
func RenderMarkdown(fs embed.FS, file string) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		data, err := fs.Open(file)
		if err != nil {
			return fmt.Errorf("unable to open document %s: %w", file, err)
		}
		policy, err := io.ReadAll(data)
		if err != nil {
			return fmt.Errorf("unable to read document %s: %w", file, err)
		}
		output := blackfriday.Run(policy, blackfriday.WithExtensions(blackfriday.AutoHeadingIDs))
		template := templates.Page("Privacy Policy - "+config.AppName, templates.Document(output))
		err = template.Render(req.Context(), res)
		if err != nil {
			return fmt.Errorf("unable to render document %s: %w", file, err)
		}
		return nil
	})).ServeHTTP
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
