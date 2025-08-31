// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/providers/elastic/query"
	"github.com/immanent-tech/go-feed-me/providers/elastic/schema"
)

// ProtectedRoutes are routes that require user authentication.
var ProtectedRoutes = []string{"/home", "/subscription", "/article", "/settings", "/search", "/user", "/view"}

// RequireUserAuth will ensure that protected routes have valid user authentication before continuing.
func RequireUserAuth(dataAPI *elastic.API, authAPI models.AuthAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			routePattern := chi.RouteContext(req.Context()).RoutePattern()
			if !slices.ContainsFunc(ProtectedRoutes, func(route string) bool {
				return strings.HasPrefix(routePattern, route)
			}) {
				next.ServeHTTP(res, req)
				return
			}
			ctx := req.Context()
			userID := authAPI.GetUserID(ctx)
			if userID == "" {
				slogctx.FromCtx(ctx).Error("Authentication Error.",
					slog.String("error", "User not found."))
				http.Redirect(res, req, "/", http.StatusSeeOther)
				return
			}
			// Fetch the user from the user management API.
			user, _, err := elastic.Search[*models.User](ctx, dataAPI.GetAPI(), schema.UsersSchemaPrefix, query.Term("external_user_id", userID), 1)
			if err != nil || len(user) == 0 {
				slogctx.FromCtx(ctx).Error("Authentication Error.",
					slog.Any("error", err))
				http.Redirect(res, req, "/", http.StatusSeeOther)
				return
			}
			// Else load the user into the context and pass the new context
			// to the next request.
			next.ServeHTTP(res, req.WithContext(models.UserToCtx(ctx, user[0])))
		})
	}
}
