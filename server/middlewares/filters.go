// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"

	"github.com/goforj/godump"
	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
)

// CanonicalizeListFilters redirects to the fully-specified, whitelisted query string if the incoming request's query
// doesn't already match it exactly.
func CanonicalizeListFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			var filters *models.ListFilters2
			if htmx.IsHistoryRestoreRequest(req) {
				// For a history restore request, fetch the filters from the session.
				filters = models.ListFiltersFromSession(req.Context())
				// Set upto to the value of from and reset from.
				upto := *filters.From
				filters.UpTo = &upto
				filters.From = nil
			} else {
				// For regular requests, parse the filters from the query. If they differ, redirect the user.
				filters = models.ParseListFilters(req.URL.Query())
				if canonical := filters.Encode(); req.URL.RawQuery != canonical {
					req.URL.RawQuery = canonical
					http.Redirect(res, req, req.URL.String(), http.StatusFound)
					return
				}
			}
			ctx := models.ListFiltersToCtx(req.Context(), filters)
			models.ListFiltersToSession(ctx, filters)
			godump.Dump(models.ListFiltersFromCtx(ctx), models.ListFiltersFromSession(ctx))
			next.ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			filters, err := forms.DecodeForm[*models.ListFilters2](req)
			switch {
			case err != nil:
				slogctx.FromCtx(req.Context()).Warn("Unable to decode list filters. Using filters from session.",
					slog.Any("error", err),
					slog.Any("filters", filters),
				)
				// Try to restore filters from session.
				ctx := models.ListFiltersToCtx(req.Context(), models.NewListFilters())
				models.ListFiltersToSession(ctx, filters)
				next.ServeHTTP(res, req.WithContext(ctx))
			default:
				ctx := models.ListFiltersToCtx(req.Context(), filters)
				models.ListFiltersToSession(ctx, filters)
				godump.Dump(models.ListFiltersFromCtx(ctx), models.ListFiltersFromSession(ctx))
				next.ServeHTTP(res, req.WithContext(ctx))
			}
		}
	})
}
