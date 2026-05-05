// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	htmxext "github.com/immanent-tech/foragd/web/htmx"

	"github.com/immanent-tech/foragd/web/templates"
)

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var userContentHandlerChain = alice.New(storePath, noCache, refreshOnHistoryRestore)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// StaticFileHandler handles serving content from the embedded filesystem containing static assets (i.e., images,
// etc.).
func StaticFileHandler(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		var file string
		switch {
		case strings.HasPrefix(req.URL.Path, "/content"):
			file = req.URL.Path
		case strings.HasPrefix(req.URL.Path, "/.well-known"):
			file = filepath.Join("/content", req.URL.Path)
		default:
			file = filepath.Join("/content", chi.URLParam(req, "*"))
		}

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
		case strings.HasSuffix(req.URL.Path, "js") || strings.HasSuffix(req.URL.Path, "css") || strings.HasSuffix(req.URL.Path, "_hs"):
			// JS/CSS/HS files are cached for 1 week.
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
			res.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
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
			slogctx.FromCtx(req.Context()).Debug("Is history restore")
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

// noCache stores the current request path in the context.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Cache-Control", "private, no-cache, max-age=0")
		next.ServeHTTP(res, req)
	})
}
