// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package handlers contains chainable handlers/middleware for routing.
package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	htmxext "github.com/immanent-tech/foragd/web/htmx"

	"github.com/immanent-tech/foragd/web/templates"
)

type Route = string

var (
	// ErrInvalidContent indicates that the content for rendering is invalid.
	ErrInvalidContent = errors.New("invalid content")
	// ErrInvalidRequestParams indicates that the request parameters received were invalid.
	ErrInvalidRequestParams = errors.New("invalid request parameters")
)

var internalPageHandlerChain = alice.New(storePath, withFromPath, noCache, refreshOnHistoryRestore)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
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

func withFromPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		from := req.URL.Query().Get("from")
		if from == "" {
			from = req.Header.Get("Referer")
		}
		if !isSafeLocalPath(from) {
			from = "/home"
		}
		ctx := templates.FromPathToCtx(req.Context(), from)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func isSafeLocalPath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//")
}
