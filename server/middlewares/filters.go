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
func CanonicalizeListFilters(session handlers.SessionManager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			spanCtx, span := tracer.Start(req.Context(), "canonicalize-filters")
			defer span.End()
			// Set a canonical path, used for context/session key suffixes.
			var path string
			switch {
			case strings.HasPrefix(req.URL.Path, "/list/subscriptions") || strings.HasPrefix(req.URL.Path, "/subscriptions"):
				path = "/list/subscriptions"
			case strings.HasPrefix(req.URL.Path, "/list/articles") || strings.HasPrefix(req.URL.Path, "/articles"):
				path = "/list/articles"
			case strings.HasPrefix(req.URL.Path, "/map"):
				path = "/map"
			}

			// When not on list pages, just load the filters from the session into the context.
			if req.Method == http.MethodGet && !strings.HasSuffix(req.URL.Path, "/articles") &&
				!strings.HasSuffix(req.URL.Path, "/subscriptions") && !strings.HasPrefix(req.URL.Path, "/map") {
				filters := handlers.ListFiltersFromSession(spanCtx, session, path)
				// Reset pagination.
				filters.From = nil
				filters.UpTo = nil
				// Store in context.
				ctx := models.ListFiltersToCtx(req.Context(), filters)
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}

			switch req.Method {
			case http.MethodGet:
				var filters *models.ListFilters
				if htmx.IsHistoryRestoreRequest(req) || req.Header.Get(models.ActionHeader) == "mark-subscription" {
					// For a history restore request, fetch the filters from the session.
					filters = handlers.ListFiltersFromSession(spanCtx, session, path)
					switch {
					case filters.From != nil:
						// Set upto as the value of from and reset from.
						upto := *filters.From
						filters.UpTo = &upto
						filters.From = nil
					case filters.SearchAfter != nil:
						// Set upto from value stored in session.
						count := handlers.ListCountFromSession(spanCtx, session, path)
						filters.UpTo = &count
						filters.SearchAfter = nil
					}
				} else {
					// For regular requests, parse the filters from the query.
					filters = models.ParseListFilters(req.URL.Query())
					if canonical := filters.Encode(); req.URL.RawQuery != canonical {
						slogctx.Debug(spanCtx, "Redirect after filters canonicalization.",
							slog.String("query", req.URL.RawQuery),
							slog.String("canonical", canonical))
						req.URL.RawQuery = canonical
						http.Redirect(res, req, req.URL.String(), http.StatusFound)
						return
					}
				}
				// Save values.
				ctx := models.ListFiltersToCtx(req.Context(), filters)
				handlers.ListFiltersToSession(ctx, session, path, filters)
				handlers.ListCountToSession(ctx, session, path, filters.Count)
				next.ServeHTTP(res, req.WithContext(ctx))
			case http.MethodPost:
				filters, err := forms.DecodeForm[*models.ListFilters](req)
				if err != nil || filters == nil {
					// Try to restore filters from session.
					filters = handlers.ListFiltersFromSession(spanCtx, session, path)
					slogctx.FromCtx(spanCtx).Warn("Unable to decode list filters. Using filters from session.",
						slog.Any("error", err),
						slog.Any("filters", filters),
					)
				}
				// For pagination requests, update count in session.
				if strings.HasSuffix(req.URL.Path, "paginate") {
					count := handlers.ListCountFromSession(spanCtx, session, path)
					count += filters.Count
					handlers.ListCountToSession(spanCtx, session, path, count)
				}
				// Save values.
				ctx := models.ListFiltersToCtx(req.Context(), filters)
				handlers.ListFiltersToSession(ctx, session, path, filters)

				next.ServeHTTP(res, req.WithContext(ctx))
			}
		})
	}
}
