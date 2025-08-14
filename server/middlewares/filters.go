// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/server/session"
)

// AddFilters middleware will handle adding either saved or default filters when none are
// present.
func AddFilters() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Ignore non subscription routes.
			err := req.ParseForm()
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Could not parse request form data.",
					slog.Any("error", err))
			}
			if len(req.Form) != 0 {
				next.ServeHTTP(res, req)
				return
			}
			switch chi.RouteContext(req.Context()).RoutePattern() {
			case "/subscriptions":
				filters := session.SubscriptionFiltersFromSession(req.Context())
				http.Redirect(res, req, "/subscriptions?"+filters.Query(), http.StatusTemporaryRedirect)
			case "/articles":
				filters := session.ArticleFiltersFromSession(req.Context())
				http.Redirect(res, req, "/articles?"+filters.Query(), http.StatusTemporaryRedirect)
			default:
				next.ServeHTTP(res, req)
				return
			}
		})
	}
}
