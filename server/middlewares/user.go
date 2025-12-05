// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/session"
)

// UnprotectedRoutes are routes that DO NOT require authentication. All other routes are assumed to require authentication.
var UnprotectedRoutes = []string{"/login", "/tos", "/policies", "/img-proxy", "/content"}

// RequireUserAuth will ensure that protected routes have valid user authentication before continuing.
func RequireUserAuth(dataAPI *elastic.API) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			routePattern := chi.RouteContext(req.Context()).RoutePattern()
			// Landing page always unauthenticated.
			if routePattern == "/" {
				next.ServeHTTP(res, req)
			}
			// Continue for unprotected routes.
			if slices.ContainsFunc(
				UnprotectedRoutes,
				func(route string) bool { return strings.HasPrefix(routePattern, route) },
			) {
				next.ServeHTTP(res, req)
				return
			}
			ctx := req.Context()
			profile, ok := session.Manager.Get(ctx, "profile").(auth0.UserProfile)
			switch {
			case !ok: // Invalid session profile data.
				slogctx.FromCtx(ctx).Error("Authentication Error.",
					slog.String("error", "invalid session profile data"))
				if htmx.IsHTMX(req) {
					res.Header().Set(htmx.HeaderRedirect, "/")
				} else {
					http.Redirect(res, req, "/", http.StatusTemporaryRedirect)
				}
				return
			case profile.Blocked: // Account is blocked.
				slogctx.FromCtx(ctx).Error("Authentication Error.",
					slog.String("error", "account is blocked"))
				if htmx.IsHTMX(req) {
					res.Header().Set(htmx.HeaderRedirect, models.RouteUserAccountIssue)
				} else {
					http.Redirect(res, req, models.RouteUserAccountIssue, http.StatusTemporaryRedirect)
				}
				return
			}
			// Fetch the user from the user management API.
			user, err := dataAPI.FindUserByExternalID(ctx, profile.GetID())
			if err != nil {
				slogctx.FromCtx(ctx).Error("Authentication Error.",
					slog.Any("error", err))
				if htmx.IsHTMX(req) {
					res.Header().Set(htmx.HeaderRedirect, "/")
				} else {
					http.Redirect(res, req, "/", http.StatusTemporaryRedirect)
				}
				return
			}
			// Else load the user into the context and pass the new context
			// to the next request.
			next.ServeHTTP(res, req.WithContext(models.UserToCtx(ctx, user)))
		})
	}
}
