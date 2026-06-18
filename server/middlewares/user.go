// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package middlewares

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/paddle"
	"github.com/immanent-tech/foragd/server/otel"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/service"
)

// ExtractUserFromSession will extract the user data from the session, retrieve the user details from the backend and
// then store the user object in the context for use by later handlers.
func ExtractUserFromSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if otel.IsEnabled() {
			_, span := otel.TracerProvider.Tracer("").
				Start(req.Context(), "require-user-auth")
			defer span.End()
		}

		// Ignore updates route.
		if strings.HasPrefix(req.URL.Path, "/updates") {
			next.ServeHTTP(res, req)
			return
		}

		// If user isn't authenticated, redirect to authenticate.
		if !auth0.IsAuthenticated(req) {
			slogctx.FromCtx(req.Context()).Warn("Unauthenticated; redirecting to login.")
			auth0.PutReturnTo(req, req.URL.RequestURI())
			if htmx.IsHTMX(req) {
				res.Header().Add(htmx.HeaderRedirect, "/login")
				res.WriteHeader(http.StatusUnauthorized)
				return
			}
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
				if htmx.IsHTMX(req) {
					res.Header().Add(htmx.HeaderRedirect, "/login")
					res.WriteHeader(http.StatusUnauthorized)
					return
				}
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
				if htmx.IsHTMX(req) {
					res.Header().Add(htmx.HeaderRedirect, "/login")
					res.WriteHeader(http.StatusUnauthorized)
					return
				}
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
			if htmx.IsHTMX(req) {
				res.Header().Add(htmx.HeaderRedirect, "/login")
				res.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(res, req, "/login", http.StatusFound)
			return
		}

		if profile.Blocked {
			slogctx.FromCtx(req.Context()).
				Error("Attempted access from blocked user. Redirecting to account issue page.",
					slog.String("external_user_id", profile.GetID()),
				)
			if htmx.IsHTMX(req) {
				res.Header().Set(htmx.HeaderRedirect, "/account-issue")
			} else {
				http.Redirect(res, req, "/account-issue", http.StatusTemporaryRedirect)
			}
			return
		}

		// Fetch the user from the user management API.
		user, err := service.GetUserByExternalID(req.Context(), profile.GetID())
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

		// Add context values.
		ctx := models.UserToCtx(req.Context(), user)
		ctx = slogctx.With(ctx, slog.String("user_id", user.GetID()))

		// Pass to next request.
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// RequireValidUser will ensure that protected routes have a valid user status before continuing.
func RequireValidUser(next http.Handler) http.Handler {
	paddleHandler := validatePaddleSubscription(next)
	androidHandler := validateAndroidSubscription(next)

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if otel.IsEnabled() {
			_, span := otel.TracerProvider.Tracer("").Start(req.Context(), "require-valid-user")
			defer span.End()
		}

		user := models.UserFromCtx(req.Context())
		if user == nil {
			res.WriteHeader(http.StatusForbidden)
			return
		}

		switch {
		case user.Metadata.Blocked:
			// User is blocked. Do not continue.
			slogctx.FromCtx(req.Context()).Error("Blocked user.")
			res.WriteHeader(http.StatusForbidden)
			return

		case !user.Metadata.PoliciesAccepted:
			// User has not accepted policies, redirect to page asking them to contact support.
			slogctx.FromCtx(req.Context()).Error("User has not accepted policies.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case !user.InTrial() && user.HasValidSubscription():
			// User not in trial and has a subscription.
			switch *user.UserSubscriptionType {
			case models.UserSubscriptionTypePaddle:
				paddleHandler.ServeHTTP(res, req)
				return
			case models.UserSubscriptionTypeAndroid:
				androidHandler.ServeHTTP(res, req)
				return
			default:
				res.WriteHeader(http.StatusForbidden)
				return
			}

		case user.InTrialGracePeriod():
			// Trial grace period. User can still use the app but will see a permanent (dismissable) notification that
			// they need to buy a subscription.
			slogctx.FromCtx(req.Context()).Warn("User in trial grace period.")

		default:
			// ! While Android app is in beta, allow Android app users to continue using the app after their trial has
			// ! expired.
			if client := models.ClientTypeFromCtx(req.Context()); client != models.ClientTypeTwa && !user.InTrial() {
				slogctx.FromCtx(req.Context()).Error("Trial expired. User account requires activation.")
				ctx := models.UserToCtx(req.Context(), user)
				http.Redirect(res, req.WithContext(ctx), "/checkout", http.StatusSeeOther)
				return
			} else {
				slogctx.FromCtx(req.Context()).Info("Android app using out of trial.")
			}
		}

		next.ServeHTTP(res, req)
	})
}

// validatePaddleSubscription performs steps necessary to validate a user's Paddle subscription.
func validatePaddleSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			res.WriteHeader(http.StatusForbidden)
			return
		}

		userSubscription, err := user.Subscription.AsPaddleSubscription()
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Get user paddle subscription failed.",
				slog.Any("error", err),
			)
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return
		}
		switch {
		case paddle.IsPastDue(&userSubscription):
			// User account is past due or has other payment issues.
			slogctx.FromCtx(req.Context()).Error("User account is past due.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case paddle.IsPaused(&userSubscription):
			slogctx.FromCtx(req.Context()).Error("User account is paused.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case paddle.IsCancelled(&userSubscription):
			// User subscription is cancelled.
			slogctx.FromCtx(req.Context()).Error("User has cancelled account.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case !paddle.IsActive(&userSubscription):
			// User subscription is cancelled.
			slogctx.FromCtx(req.Context()).Error("User account requires activation.")
			ctx := models.UserToCtx(req.Context(), user)
			http.Redirect(res, req.WithContext(ctx), "/checkout", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(res, req)
	})
}

// validateAndroidSubscription performs steps necessary to validate a user's Android subscription.
func validateAndroidSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		next.ServeHTTP(res, req)
	})
}
