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
			// Optimized: pre-compute path once, avoids repeated string checks in nested branches.
			path := determinePath(req.URL.Path)

			if !isListPage(req.URL.Path) {
				slogctx.Debug(
					req.Context(),
					"Non-list page, using filters from session.",
					slog.String("path", req.URL.Path),
				)
				// Early return for non-list pages - avoids all filter processing.
				filters := handlers.ListFiltersFromSession(req.Context(), session, path)
				filters.From = nil
				filters.UpTo = nil
				ctx := handlers.ListFiltersToCtx(req.Context(), *filters)
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}

			filters, _, redirect := prepareFilters(req, path, session)
			if redirect {
				// Optimization: redirect without span context (already captured).
				req.URL.RawQuery = filters.Encode()
				http.Redirect(res, req, req.URL.String(), http.StatusFound)
				return
			}

			// Process filters based on request method.
			if req.Method == http.MethodPost {
				filters = processFilters(req, path, session, res)
			}

			// Single context setup - all operations below.
			ctx := handlers.ListFiltersToCtx(req.Context(), *filters)
			handlers.ListFiltersToSession(ctx, session, path, *filters)
			handlers.ListCountToSession(ctx, session, path, filters.Count)
			if req.Method == http.MethodGet {
				pushURL := req.URL.Clone()
				pushURL.Path = path
				pushURL.RawQuery = filters.Encode()
				res.Header().Set(htmx.HeaderPushURL, pushURL.String())
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// determinePath extracts the canonical path from URL for filter key suffixes.
// Uses fast switch over known patterns instead of repeated HasPrefix/HasSuffix checks.
func determinePath(urlPath string) string {
	switch urlPath {
	case "/subscriptions", "/subscriptions/":
		return "/subscriptions"
	case "/articles", "/articles/":
		return "/articles"
	case "/map", "/map/":
		return "/map"
	default:
		return ""
	}
}

// isListPage determines if URL path is a list page that requires filter processing.
func isListPage(urlPath string) bool {
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

// prepareFilters extracts, sanitizes, and validates filters based on request type.
// Returns filters pointer, whether to fetch from session, and whether to redirect.
func prepareFilters(req *http.Request, path string, session handlers.SessionManager) (*models.ListFilters, bool, bool) {
	if req.Method != http.MethodGet {
		// POST: will be handled by caller (session decode logic below)
		return nil, false, false
	}

	// History restore or mark-subscription: fetch from session
	if htmx.IsHistoryRestoreRequest(req) || req.Header.Get(models.ActionHeader) == "mark-subscription" {
		filters := handlers.ListFiltersFromSession(req.Context(), session, path)
		if filters.From != nil {
			upto := *filters.From
			filters.UpTo = &upto
			filters.From = nil
		} else if filters.SearchAfter != nil {
			count := handlers.ListCountFromSession(req.Context(), session, path)
			filters.UpTo = &count
			filters.SearchAfter = nil
		}
		return filters, true, false
	}

	// Regular GET: parse from query string
	filters := models.ParseListFilters(req.URL.Query())
	filters.Sanitise()

	// Check canonicalization - redirect if query changes
	canonical := filters.Encode()
	return filters, false, req.URL.RawQuery != canonical
}

// processFilters handles POST requests and session storage.
func processFilters(
	req *http.Request,
	path string,
	session handlers.SessionManager,
	res http.ResponseWriter,
) *models.ListFilters {
	filters, err := forms.DecodeForm[*models.ListFilters](req)
	if err != nil || filters == nil {
		filters = handlers.ListFiltersFromSession(req.Context(), session, path)
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Unable to decode list filters. Using filters from session.",
				slog.Any("error", err),
				slog.Any("filters", filters),
			)
		}
	}
	// For pagination requests, update count in session.
	if strings.HasSuffix(req.URL.Path, "paginate") {
		count := handlers.ListCountFromSession(req.Context(), session, path)
		count += filters.Count
		handlers.ListCountToSession(req.Context(), session, path, count)
	}
	return filters
}
