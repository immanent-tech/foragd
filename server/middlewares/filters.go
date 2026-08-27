// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

// CanonicalizeListFilters handles processing and storing the fully-specified, whitelisted list filters for the user.
func CanonicalizeListFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Set a canonical path, used for context/session key suffixes.
		var path string
		switch {
		case strings.HasPrefix(req.URL.Path, "/list/subscriptions"):
			path = "/list/subscriptions"
		case strings.HasPrefix(req.URL.Path, "/list/articles"):
			path = "/list/articles"
		}
		switch req.Method {
		case http.MethodGet:
			var filters *models.ListFilters
			if htmx.IsHistoryRestoreRequest(req) {
				// For a history restore request, fetch the filters from the session.
				filters = models.ListFiltersFromSession(req.Context(), path)
				switch {
				case filters.From != nil:
					// Set upto as the value of from and reset from.
					upto := *filters.From
					filters.UpTo = &upto
					filters.From = nil
				case filters.SearchAfter != nil:
					// Set upto from value stored in session.
					count := models.ListCountFromSession(req.Context(), path)
					filters.UpTo = &count
					filters.SearchAfter = nil
				}
			} else {
				// For regular requests, parse the filters from the query. If they differ, redirect the user.
				filters = models.ParseListFilters(req.URL.Query())
				if canonical := filters.Encode(); req.URL.RawQuery != canonical {
					slogctx.Debug(req.Context(), "Redirect after filters canonicalization.",
						slog.String("query", req.URL.RawQuery),
						slog.String("canonical", canonical))
					req.URL.RawQuery = canonical
					http.Redirect(res, req, req.URL.String(), http.StatusFound)
					return
				}
			}
			// Save values.
			ctx := models.ListFiltersToCtx(req.Context(), filters)
			models.ListFiltersToSession(ctx, path, filters)
			models.ListCountToSession(ctx, path, filters.Count)
			next.ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			filters, err := forms.DecodeForm[*models.ListFilters](req)
			if err != nil {
				// Try to restore filters from session.
				filters = models.ListFiltersFromSession(req.Context(), path)
				slogctx.FromCtx(req.Context()).Warn("Unable to decode list filters. Using filters from session.",
					slog.Any("error", err),
					slog.Any("filters", filters),
				)
			}
			// For pagination requests, update count in session.
			if strings.HasSuffix(req.URL.Path, "paginate") {
				count := models.ListCountFromSession(req.Context(), path)
				count += filters.Count
				models.ListCountToSession(req.Context(), path, count)
			}
			// Save values.
			ctx := models.ListFiltersToCtx(req.Context(), filters)
			models.ListFiltersToSession(ctx, path, filters)

			next.ServeHTTP(res, req.WithContext(ctx))
		}
	})
}
