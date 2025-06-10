// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
)

// ProtectedRoutes are routes that require user authentication.
var ProtectedRoutes = []string{"/home", "/subscription", "/article", "/settings", "/search"}

// PerformAuth will perform authentication for a user with a provider.
func PerformAuth(api AuthAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		slogctx.FromCtx(req.Context()).Debug("Authenticating user.")
		if err := api.CompleteUserAuth(res, req); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Authentication required.", slog.Any("error", err))
			url, err := api.GetAuthURL(req)
			if err != nil {
				ProcessResponse(res, req, &models.Response{
					StatusCode:    http.StatusInternalServerError,
					InternalError: err,
					UserMessage: &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "Authentication failed.",
					},
				})
				return
			}
			slogctx.FromCtx(req.Context()).Debug("Redirecting to provider.", slog.String("url", url))
			http.Redirect(res, req, url, http.StatusTemporaryRedirect)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("Redirecting to home page.")
		req.Header.Add("Content-Type", "")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	})
}

// AuthCallback handles a callback from an authentication provider.
func AuthCallback(authAPI AuthAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if err := authAPI.CompleteUserAuth(res, req); err != nil {
			ProcessResponse(res, req, &models.Response{
				StatusCode:    http.StatusInternalServerError,
				InternalError: err,
				UserMessage: &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Authentication failed.",
				},
			})
			return
		}
		slogctx.FromCtx(req.Context()).Debug("Authenticated.")
		slogctx.FromCtx(req.Context()).Debug("Redirecting to home page.")
		req.Header.Add("Content-Type", "")

		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	})
}

// RequireUserAuth will ensure that protected routes have valid user authentication before continuing.
func RequireUserAuth(dataAPI UserAPI, authAPI AuthAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			routePattern := chi.RouteContext(req.Context()).RoutePattern()
			if !slices.ContainsFunc(ProtectedRoutes, func(route string) bool {
				return strings.HasPrefix(routePattern, route)
			}) {
				slogctx.FromCtx(req.Context()).Debug("Route does not require auth.",
					slog.String("route", routePattern))
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
			ctx = elastic.UserIndexToCtx(ctx, schema.UsersSchemaPrefix)
			// Fetch the user from the user management API.
			user, err := dataAPI.GetUser(ctx, userID)
			//  If no user can be found, redirect back to the home page.
			if err != nil {
				slogctx.FromCtx(ctx).Error("Authentication Error.",
					slog.Any("error", err))
				http.Redirect(res, req, "/", http.StatusSeeOther)
				return
			}
			// Else load the user into the context and pass the new context
			// to the next request.
			next.ServeHTTP(res, req.WithContext(models.UserToCtx(ctx, user)))
		})
	}
}
