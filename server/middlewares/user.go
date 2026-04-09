// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/oauth2"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/server/session"
)

// RequireUserAuth will ensure that protected routes have valid user authentication before continuing.
func RequireUserAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/updates") {
			next.ServeHTTP(res, req)
			return
		}

		ctx := req.Context()

		// Validate the access token stored in the session.
		if token, err := session.Restore[oauth2.Token](req.Context(), "token"); err != nil || !token.Valid() {
			if !strings.HasSuffix(req.URL.Path, "/updates") {
				slogctx.FromCtx(req.Context()).
					Error("Invalid session token. Generating new state and redirecting to login.",
						slog.Any("error", err),
					)
				// Generate new state and save url for redirection after login.
				if state, err := auth0.GenerateRandomState(); err != nil {
					slogctx.FromCtx(req.Context()).Error("Generate new state failed.",
						slog.Any("error", err),
					)
				} else {
					session.Save(req.Context(), "state", state)
					session.Save(req.Context(), state, map[string]string{
						"redirectURL": req.URL.String(),
					})
				}
				http.Redirect(res, req, "/login", http.StatusSeeOther)
				return
			}
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

		// Add otel attributes.
		_, span := otel.TracerProvider.Tracer("").
			Start(ctx, "RequireUserAuth", trace.WithAttributes(attribute.String("id", user.GetID())))
		defer span.End()

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
			slogctx.FromCtx(req.Context()).
				Error("Invalid session token. Unable to refresh. Generating new state and redirecting to login.",
					slog.Any("error", err),
				)
			// Generate new state and save url for redirection after login.
			if state, err := auth0.GenerateRandomState(); err != nil {
				slogctx.FromCtx(req.Context()).Error("Generate new state failed.",
					slog.Any("error", err),
				)
			} else {
				session.Save(req.Context(), "state", state)
				session.Save(req.Context(), state, map[string]string{
					"redirectURL": req.URL.String(),
				})
			}
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Check token validity.
		if !token.Valid() {
			slogctx.FromCtx(req.Context()).Error("Invalid user token.")
			res.WriteHeader(http.StatusForbidden)
			return
		}

		// If token will expire soon, refresh it.
		const refreshGracePeriod = time.Hour
		if token.Expiry.UTC().Sub(time.Now().UTC()) < refreshGracePeriod {
			newToken, err := auth0.RefreshAccessToken(res, req, &token)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Unable to refresh token.",
					slog.Any("error", err),
				)
				http.Redirect(res, req, "/login", http.StatusSeeOther)
				return
			}

			// Save the new token into the session data.
			session.Save(req.Context(), "token", *newToken)
			// Renew the session data.
			if err := session.Renew(req.Context()); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to renew session data.",
					slog.Any("error", err),
				)
			}

			// Commit the session to the store.
			if err := session.Commit(req.Context()); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to commit session data.",
					slog.Any("error", err),
				)
				http.Redirect(res, req, "/login", http.StatusSeeOther)
				return
			}
		}

		next.ServeHTTP(res, req)
	})
}
