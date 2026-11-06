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

			log := slogctx.FromCtx(req.Context())

			if !IsListPage(req.URL.Path) {
				// Early return for non-list pages - avoids all filter processing.
				filters := models.NewListFilters()
				ctx := handlers.ListFiltersToCtx(req.Context(), *filters)
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}

			var filters *models.ListFilters

			// Process filters based on request method.
			switch {
			case htmx.IsHistoryRestoreRequest(req) || req.Header.Get(models.ActionHeader) == "mark-subscription":
				// History restore or mark-subscription: fetch from session
				filters = handlerFiltersRestore(req, path, session)
			case req.Method == http.MethodGet:
				var redirect bool
				filters, redirect = handleFiltersGet(req)
				if redirect {
					// Optimization: redirect without span context (already captured).
					req.URL.RawQuery = filters.Encode()
					http.Redirect(res, req, req.URL.String(), http.StatusFound)
					return
				}
			case req.Method == http.MethodPost:
				filters = handleFiltersPost(req, path, session)
			}

			// Single context setup - all operations below.
			log.Debug(
				"Saving filters",
				slog.String("filters", filters.Encode()),
				slog.String("path", path),
			)

			ctx := handlers.ListFiltersToCtx(req.Context(), *filters)
			handlers.ListFiltersToSession(ctx, session, path, *filters)
			if filters.UpTo != nil {
				handlers.ListCountToSession(ctx, session, path, *filters.UpTo)
			} else {
				handlers.ListCountToSession(ctx, session, path, filters.Count)
			}
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
	if filters.From != nil {
		upto := *filters.From + filters.Count
		filters.UpTo = &upto
		filters.From = nil
	} else if filters.SearchAfter != nil {
		count := handlers.ListCountFromSession(req.Context(), session, path)
		upto := count + filters.Count
		filters.UpTo = &upto
		filters.SearchAfter = nil
	}
	return filters
}

// handleFiltersGet extracts, sanitizes, and validates filters based on request type. Returns filters pointer and
// whether to redirect.
func handleFiltersGet(
	req *http.Request,
) (*models.ListFilters, bool) {
	log := slogctx.FromCtx(req.Context())

	// Regular GET: parse from query string
	filters := models.ParseListFilters(req.URL.Query())
	if err := filters.Sanitise(); err != nil {
		log.Warn("Filter sanitization failed.", slog.Any("error", err))
	}

	// Check canonicalization - redirect if query changes
	canonical := filters.Encode()
	return filters, req.URL.RawQuery != canonical
}

// handleFiltersPost handles POST requests and session storage.
func handleFiltersPost(
	req *http.Request,
	path string,
	session handlers.SessionManager,
) *models.ListFilters {
	log := slogctx.FromCtx(req.Context())

	filters, err := forms.DecodeForm[*models.ListFilters](req)
	if err != nil || filters == nil {
		filters = handlers.ListFiltersFromSession(req.Context(), session, path)
		if err != nil {
			log.Warn("Unable to decode list filters. Using filters from session.",
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
