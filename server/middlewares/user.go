// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/oauth2"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/session"
)

// RequireUserAuth will ensure that protected routes have valid user authentication before continuing.
func RequireUserAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		// Validate the access token stored in the session.
		if token, err := session.Restore[oauth2.Token](req.Context(), "token"); err != nil || !token.Valid() {
			slogctx.FromCtx(req.Context()).Error("Invalid session token.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusForbidden)
			return
		}

		profile, err := session.Restore[auth0.UserProfile](ctx, "profile")
		switch {
		case err != nil: // Invalid session profile data.
			slogctx.FromCtx(ctx).Error("Authentication Error.",
				slog.String("external_user_id", profile.GetID()),
				slog.Any("error", err))
			if htmx.IsHTMX(req) {
				res.Header().Set(htmx.HeaderRedirect, "/")
			} else {
				http.Redirect(res, req, "/", http.StatusTemporaryRedirect)
			}
			return
		case profile.Blocked: // Account is blocked.
			slogctx.FromCtx(ctx).Error("Authentication Error.",
				slog.String("external_user_id", profile.GetID()),
				slog.String("error", "account is blocked"))
			if htmx.IsHTMX(req) {
				res.Header().Set(htmx.HeaderRedirect, models.RouteUserAccountIssue)
			} else {
				http.Redirect(res, req, models.RouteUserAccountIssue, http.StatusTemporaryRedirect)
			}
			return
		}
		// Fetch the user from the user management API.
		user, err := models.GetUserByExternalID(ctx, profile.GetID())
		if err != nil {
			slogctx.FromCtx(ctx).Error("Authentication Error.",
				slog.String("external_user_id", profile.GetID()),
				slog.Any("error", err))
			if htmx.IsHTMX(req) {
				res.Header().Set(htmx.HeaderRedirect, "/")
			} else {
				http.Redirect(res, req, "/", http.StatusTemporaryRedirect)
			}
			return
		}
		// Do not continue if user is blocked.
		if user.Metadata.Blocked {
			res.WriteHeader(http.StatusForbidden)
			return
		}
		// Add context values.
		ctx = models.UserToCtx(ctx, user)
		ctx = slogctx.With(ctx, slog.String("user_id", user.GetID()))

		// Pass to next request.
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// RefreshTokenIfNeeded handles refreshing the user's access token (using a refresh token) when it is about to expire.
func RefreshTokenIfNeeded(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the refresh token from the session.
		token, err := session.Restore[oauth2.Token](req.Context(), "token")
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Invalid session token.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusForbidden)
			return
		}
		// Check token validity.
		if !token.Valid() {
			slogctx.FromCtx(req.Context()).Error("Invalid user token.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusForbidden)
			return
		}

		const refreshGracePeriod = time.Hour

		// If token is about to expire, refresh it.
		if token.Expiry.UTC().Sub(time.Now().UTC()) < refreshGracePeriod {
			if err := auth0.RefreshAccessToken(req, &token); err != nil {
				slogctx.FromCtx(req.Context()).Error("Unable to refresh token.",
					slog.Any("error", err),
				)
			}
		}

		next.ServeHTTP(res, req)
	})
}
