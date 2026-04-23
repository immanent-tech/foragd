// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/server/session"
)

// RequireUserAuth will ensure that protected routes have valid user authentication before continuing.
func RequireUserAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Ignore updates route.
		if strings.HasPrefix(req.URL.Path, "/updates") {
			next.ServeHTTP(res, req)
			return
		}

		// If user isn't authenticated, redirect to authenticate.
		if !auth0.IsAuthenticated(req) {
			slogctx.FromCtx(req.Context()).Warn("Unauthenticated; redirecting to login.")
			auth0.PutReturnTo(req, req.URL.RequestURI())
			http.Redirect(res, req, "/login", http.StatusFound)
			return
		}

		if auth0.IsAccessTokenExpired(req) {
			refreshToken, err := auth0.GetRefreshToken(req)
			if err != nil || refreshToken == "" {
				slogctx.FromCtx(req.Context()).Warn("Access token expired and no refresh token; redirecting to login.",
					slog.Any("error", err),
				)
				auth0.ClearAuth(req)
				auth0.PutReturnTo(req, req.URL.RequestURI())
				http.Redirect(res, req, "/login", http.StatusFound)
				return
			}

			slogctx.FromCtx(req.Context()).Debug("Access token expired; attempting refresh.")
			token, err := auth0.RefreshTokens(req.Context(), refreshToken)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Token refresh failed.",
					slog.Any("error", err),
				)
				auth0.ClearAuth(req)
				auth0.PutReturnTo(req, req.URL.RequestURI())
				http.Redirect(res, req, "/login", http.StatusFound)
				return
			}

			// Rotate tokens in session.
			auth0.SaveTokens(req.Context(), token)
			slogctx.FromCtx(req.Context()).Debug("Token refresh successful.")
		}

		profile, err := session.Restore[auth0.UserProfile](req.Context(), "profile")
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Unable to retrieve profile from session.",
				slog.Any("error", err),
			)
			auth0.ClearAuth(req)
			auth0.PutReturnTo(req, req.URL.RequestURI())
			http.Redirect(res, req, "/login", http.StatusFound)
			return
		}

		if profile.Blocked {
			slogctx.FromCtx(req.Context()).
				Error("Attempted access from blocked user. Redirecting to account issue page.",
					slog.String("external_user_id", profile.GetID()),
				)
			if htmx.IsHTMX(req) {
				res.Header().Set(htmx.HeaderRedirect, models.RouteUserAccountIssue)
			} else {
				http.Redirect(res, req, models.RouteUserAccountIssue, http.StatusTemporaryRedirect)
			}
			return
		}

		// Fetch the user from the user management API.
		user, err := models.GetUserByExternalID(req.Context(), profile.GetID())
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Get local user data failed.",
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
		ctx := models.UserToCtx(req.Context(), user)
		ctx = slogctx.With(ctx, slog.String("user_id", user.GetID()))

		// Add otel attributes.
		_, span := otel.TracerProvider.Tracer("").
			Start(ctx, "RequireUserAuth", trace.WithAttributes(attribute.String("id", user.GetID())))
		defer span.End()

		// Pass to next request.
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}
