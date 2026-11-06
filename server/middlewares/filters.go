/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/server/handlers"
)

// CanonicalizeListFilters handles processing and storing the fully-specified, whitelisted list filters for the user.
// Optimized for performance: reduces tracing allocations, flattens handler chain, and minimizes string operations.
func CanonicalizeListFilters(session handlers.SessionManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Optimized: precompute path once, avoids repeated string checks in nested branches.
			path := determinePath(req.URL.Path)

			if !IsListPage(req.URL.Path) {
				// Early return for non-list pages - avoids all filter processing.
				filters := models.NewListFilters()
				ctx := handlers.ListFiltersToCtx(req.Context(), *filters)
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}

			var (
				filters   *models.ListFilters
				urlHeader string
			)

			switch {
			case htmx.IsHistoryRestoreRequest(req) || req.Header.Get(models.ActionHeader) == "mark-subscription":
				// History restore or mark-subscription: fetch from session
				filters = handlerFiltersRestore(req, path, session)
				urlHeader = htmx.HeaderPushURL
			case req.Method == http.MethodGet:
				// Get request.
				var redirect bool
				filters, redirect = handleFiltersGet(req)
				if redirect {
					req.URL.RawQuery = filters.Encode()
					http.Redirect(res, req, req.URL.String(), http.StatusFound)
					return
				}
				urlHeader = htmx.HeaderPushURL
			case req.Method == http.MethodPost:
				// Post request (pagination).
				filters = handleFiltersPost(req, path, session)
				urlHeader = htmx.HeaderReplaceUrl
			}

			// Store filters.
			ctx := handlers.ListFiltersToCtx(req.Context(), *filters)
			handlers.ListFiltersToSession(ctx, session, path, *filters)

			// Update URL.
			newURL := req.URL.Clone()
			newURL.Path = path
			newURL.RawQuery = filters.Encode()
			res.Header().Set(urlHeader, newURL.String())

			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// IsListPage determines if URL path is a list page that requires filter processing.
func IsListPage(urlPath string) bool {
	switch urlPath {
	case "/articles", "/articles/paginate", "/articles/updates":
		return true
	case "/subscriptions", "/subscriptions/paginate", "/subscriptions/updates":
		return true
	case "/map", "/map/updates":
		return true
	default:
		return false
	}
}

// determinePath extracts the canonical path from URL for filter key suffixes.
func determinePath(urlPath string) string {
	switch {
	case strings.HasPrefix(urlPath, "/subscriptions"):
		return "/subscriptions"
	case strings.HasPrefix(urlPath, "/articles"):
		return "/articles"
	case strings.HasPrefix(urlPath, "/map"):
		return "/map"
	default:
		return ""
	}
}

func handlerFiltersRestore(req *http.Request, path string, session handlers.SessionManager) *models.ListFilters {
	filters := handlers.ListFiltersFromSession(req.Context(), session, path)
	slogctx.Debug(req.Context(),
		"Restored filters from session.",
		slog.String("filters", filters.Encode()),
		slog.String("path", path),
	)
	// Reset parameters.
	count := filters.GetFrom()
	filters.Count = &count
	filters.From = 0
	filters.SearchAfter = nil
	return filters
}

func handleFiltersGet(
	req *http.Request,
) (*models.ListFilters, bool) {
	log := slogctx.FromCtx(req.Context())

	// Regular GET: parse from query string
	filters := models.ParseListFilters(req.URL.Query())
	if err := filters.Sanitise(); err != nil {
		log.Warn("Filter sanitization failed.", slog.Any("error", err))
	}

	// Reset parameters on non-htmx requests.
	if !htmx.IsHTMX(req) {
		count := filters.GetFrom()
		filters.Count = &count
		filters.From = 0
		filters.SearchAfter = nil
	}

	// Check canonicalization - redirect if query changes
	canonical := filters.Encode()
	return filters, req.URL.RawQuery != canonical
}

func handleFiltersPost(
	req *http.Request,
	path string,
	session handlers.SessionManager,
) *models.ListFilters {
	log := slogctx.FromCtx(req.Context())

	filters, err := forms.DecodeForm[*models.ListFilters](req)
	if err != nil || filters == nil {
		log.Warn("Unable to decode list filters. Using filters from session.",
			slog.Any("error", err),
			slog.Any("filters", filters),
		)
		filters = handlers.ListFiltersFromSession(req.Context(), session, path)
		if err != nil {
			log.Warn("Unable to restore filters from session.",
				slog.Any("error", err),
			)
		}
	}
	return filters
}
