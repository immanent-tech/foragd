// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/components/session"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// SetupRedirect handler will add a HX-Location header to the request when the given path is non-nil and the request has
// been made through HTMX.
func SetupRedirect(path *string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			if htmx.IsHTMX(req) && path != nil {
				var route models.Route
				switch *path {
				case "/subscriptions":
					filters := session.SubscriptionFiltersFromSession(ctx)
					route = models.NewRoute("/subscriptions", &filters)
				case "/articles":
					filters := session.ArticleFiltersFromSession(ctx)
					route = models.NewRoute("/articles", &filters)
				default:
					route = models.Route{Path: "/home"}
				}
				// Set-up client-side redirect to view.
				htmxResp := htmx.NewResponse().LocationWithContext(
					route.String(),
					htmx.LocationContext{
						Target: partials.ContentID.Target(),
					})
				ctx = context.WithValue(ctx, htmxRespCtxKey, htmxResp)
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SavePageState saves the current page state in the session.
func SavePageState(filters any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Generate state.
			session.FiltersToSession(req.Context(), filters)
			next.ServeHTTP(res, req)
		})
	}
}
