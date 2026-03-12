// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/frontmatter"

	htmxext "github.com/immanent-tech/foragd/web/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/web/templates"
)

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var defaultHandlerChain = alice.New(storePath)

var loadMarkdownWriter = sync.OnceValue(func() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			&frontmatter.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
})

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// StaticFileHandler handles serving content from the embedded filesystem containing static assets (i.e., images,
// etc.).
func StaticFileHandler(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract the file as the URL param.
		file := chi.URLParam(req, "*")
		if file == "" {
			// If no URL param, treat the last path element as a file.
			file = filepath.Base(req.URL.Path)
		}

		// Recreate the path to the file in the virtual FS.
		file = filepath.Join("/content", file)

		// Check, if the requested file is existing.
		if _, err := fs.Open(file); err != nil {
			slogctx.FromCtx(req.Context()).Error("Invalid static content request",
				slog.String("file", file),
			)
			// If file is not found, return HTTP 404 error.
			http.NotFound(res, req)
			return
		}

		// Replace the request path with the validated file path.
		req.URL.Path = file

		switch {
		case strings.HasSuffix(req.URL.Path, "js") || strings.HasSuffix(req.URL.Path, "css"):
			// JS/CSS files are cached for 1 week.
			res.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		case strings.HasSuffix(req.URL.Path, "woff2"):
			fallthrough
		case strings.HasSuffix(req.URL.Path, "png"):
			fallthrough
		case strings.HasSuffix(req.URL.Path, "jpg"):
			fallthrough
		case strings.HasSuffix(req.URL.Path, "webp"):
			fallthrough
		case strings.HasSuffix(req.URL.Path, "svg"):
			res.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		default:
			// Default is to cache for 1 week.
			res.Header().Set("Cache-Control", "public, max-age=604800, s-maxage=43200")
		}
		// File is found, return to standard http.FileServer.
		http.FileServer(fs).ServeHTTP(res, req)
	})
}

// setRedirect adds the HX-Location header with the given values to the response, which triggers a client side
// redirection without reloading the whole page.
//
// https://htmx.org/headers/hx-location/
func setRedirect(res http.ResponseWriter, request htmxext.HXLocationRequest) error {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("set redirect: marshal request: %w", err)
	}
	res.Header().Set(htmx.HeaderLocation, string(requestJSON))
	return nil
}

// refreshOnHistoryRestore will trigger a full client-side refresh on a HTMX history restore request.
func refreshOnHistoryRestore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if htmx.IsHistoryRestoreRequest(req) {
			res.Header().Set(htmx.HeaderRefresh, "true")
		}
		next.ServeHTTP(res, req)
	})
}

// storePath stores the current request path in the context.
func storePath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := templates.PathToCtx(req.Context(), req.URL.Path)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// WatchList handles watching a list of object for any updates and rendering a notification to the user to refresh the page.
func WatchList() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := models.NewListDisplayFilters()
		// // Retrieve current filters.
		// filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
		// Create a query to find new items.
		query, err := models.BuildItemsQuery(req.Context(), &filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot generate query for updates.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Watch list for updates.
		watchForUpdates(query).ServeHTTP(res, req)
	}).ServeHTTP
}

//nolint:gocognit
func watchForUpdates(watch query.Option) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			res.WriteHeader(http.StatusNoContent)
			slogctx.FromCtx(req.Context()).Error("Unable to watch for updates.",
				slog.Any("error", models.ErrCtxValueNotFound),
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
			slogctx.FromCtx(req.Context()).Error("Cannot write SSE headers.")
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set up counters.
		var (
			currentCount int64
			prevCount    int64
			err          error
		)

		// Get an initial count.
		prevCount, err = models.CountItems(req.Context(), watch)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Cannot get updates count.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Set up updatesTicker.
		updatesTicker := time.NewTicker(user.GetUpdatesFrequency())
		defer updatesTicker.Stop()
		keepAliveTicker := time.NewTicker(20 * time.Second)
		defer keepAliveTicker.Stop()
		slogctx.FromCtx(req.Context()).Debug("Checking for updates...",
			slog.Duration("interval", user.GetUpdatesFrequency()),
			slog.Group("request",
				slog.String("path", req.URL.Path),
			),
		)

		// Watch for updates.
		for {
			select {
			case <-req.Context().Done():
				slogctx.FromCtx(req.Context()).Debug("Closing SSE connection.",
					slog.Group("request",
						slog.String("path", req.URL.Path),
					),
				)
				res.WriteHeader(http.StatusOK)
				res.Header().Set("Connection", "close")
				if f, ok := res.(http.Flusher); ok {
					f.Flush()
				}
				keepAliveTicker.Stop()
				updatesTicker.Stop()
				return
			case <-updatesTicker.C:
				currentCount, err = models.CountItems(req.Context(), watch)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Cannot get updates count.",
						slog.Any("error", err))
					continue
				}
				// Show updates toast if new items found.
				if currentCount > prevCount {
					slogctx.FromCtx(req.Context()).Debug("Subscription updates found.")
					respBuf, ok := bufPool.Get().(*bytes.Buffer)
					if !ok {
						res.WriteHeader(http.StatusNoContent)
						slogctx.FromCtx(req.Context()).Error("Get response buffer failed.")
						continue
					}
					respBuf.Reset()
					defer bufPool.Put(respBuf)

					template := bufio.NewWriter(respBuf)
					if err := templates.UpdatesToast().Render(req.Context(), template); err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to render template.",
							slog.Any("error", err))
						continue
					}
					if err = template.Flush(); err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to flush SSE message buffer.",
							slog.Any("error", err))
					}
					if _, err = fmt.Fprintf(res, "event: updates\ndata: %s\n\n", respBuf.String()); err != nil {
						slogctx.FromCtx(req.Context()).Error("Failed to send update SSE message.",
							slog.Any("error", err))
					}
					if f, ok := res.(http.Flusher); ok {
						f.Flush()
					}
				}

				slogctx.FromCtx(req.Context()).Debug("No updates")
				prevCount = currentCount
			case <-keepAliveTicker.C:
				slogctx.FromCtx(req.Context()).Debug("Sending keep-alive message on SSE stream.",
					slog.Group("request",
						slog.String("path", req.URL.Path),
					),
				)
				if _, err = fmt.Fprint(res, ": keep-alive\n\n"); err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to send keep-alive SSE message.",
						slog.Any("error", err),
						slog.Group("request",
							slog.String("path", req.URL.Path),
						),
					)
				}
				if f, ok := res.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	})
}
