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

// CanonicalizeSearchParams processes and stores the search params requested by a user.
func CanonicalizeSearchParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			var search *models.SearchRequest
			if htmx.IsHistoryRestoreRequest(req) {
				// For a history restore request, fetch the params from the session.
				search = models.SearchParamsFromSession(req.Context())
				switch {
				case search.From != nil:
					// Set upto as the value of from and reset from.
					upto := *search.From
					search.UpTo = &upto
					search.From = nil
				case search.SearchAfter != nil:
					// Set upto from value stored in session.
					count := models.SearchCountFromSession(req.Context())
					search.UpTo = &count
					search.SearchAfter = nil
				}
			} else {
				// For regular requests, parse the params from the query. If they differ, redirect the user.
				search = models.ParseSearchParams(req.URL.Query())
				if canonical := search.Encode(); req.URL.RawQuery != canonical {
					slogctx.Debug(req.Context(), "Redirect after params canonicalization.",
						slog.String("query", req.URL.RawQuery),
						slog.String("canonical", canonical))
					req.URL.RawQuery = canonical
					http.Redirect(res, req, req.URL.String(), http.StatusFound)
					return
				}
			}
			// Save values.
			ctx := models.SearchParamsToCtx(req.Context(), search)
			models.SearchParamsToSession(ctx, search)
			models.SearchCountToSession(ctx, search.Count)
			next.ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			search, err := forms.DecodeForm[*models.SearchRequest](req)
			if err != nil {
				// Try to restore params from session.
				search = models.SearchParamsFromSession(req.Context())
				slogctx.FromCtx(req.Context()).Warn("Unable to decode search params. Using search params from session.",
					slog.Any("error", err),
					slog.Any("search", search),
				)
			}
			// For pagination requests, update search count in session.
			if strings.HasSuffix(req.URL.Path, "paginate") {
				count := models.SearchCountFromSession(req.Context())
				count += search.Count
				models.SearchCountToSession(req.Context(), count)
			}
			// Save values.
			ctx := models.SearchParamsToCtx(req.Context(), search)
			models.SearchParamsToSession(ctx, search)
			next.ServeHTTP(res, req.WithContext(ctx))
		}
	})
}
