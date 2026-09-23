/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/paddle"
	"github.com/immanent-tech/foragd/server/handlers"
)

type UserService interface {
	GetUserByExternalID(ctx context.Context, externalID string) (*models.User, error)
}

// ExtractUserFromSession will extract the user data from the session, retrieve the user details from the backend and
// then store the user object in the context for use by later handlers.
func ExtractUserFromSession(
	users UserService,
	auth *auth0.Authenticator,
	session handlers.SessionManager,
	httpClient *resty.Client,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

			// Ignore updates route.
			if strings.HasPrefix(req.URL.Path, "/updates") {
				next.ServeHTTP(res, req)
				return
			}

			// If user isn't authenticated, redirect to authenticate.
			if !auth.IsAuthenticated(req.Context(), session) {
				slogctx.Warn(req.Context(), "Unauthenticated; redirecting to login.")
				auth.PutReturnTo(req.Context(), session, req.URL.RequestURI())
				if htmx.IsHTMX(req) {
					res.Header().Add(htmx.HeaderRedirect, "/login")
					res.WriteHeader(http.StatusUnauthorized)
					return
				}
				http.Redirect(res, req, "/login", http.StatusFound)
				return
			}

			if auth.IsAccessTokenExpired(req.Context(), session) {
				refreshToken, err := auth.GetRefreshToken(req.Context(), session)
				if err != nil || refreshToken == "" {
					slogctx.Warn(req.Context(), "Access token expired and no refresh token; redirecting to login.",
						slog.Any("error", err),
					)
					auth.ClearAuth(req.Context(), session)
					auth.PutReturnTo(req.Context(), session, req.URL.RequestURI())
					if htmx.IsHTMX(req) {
						res.Header().Add(htmx.HeaderRedirect, "/login")
						res.WriteHeader(http.StatusUnauthorized)
						return
					}
					http.Redirect(res, req, "/login", http.StatusFound)
					return
				}

				slogctx.Debug(req.Context(), "Access token expired; attempting refresh.")
				token, err := auth.RefreshTokens(req.Context(), httpClient, refreshToken)
				if err != nil {
					slogctx.Warn(req.Context(), "Token refresh failed.",
						slog.Any("error", err),
					)
					auth.ClearAuth(req.Context(), session)
					auth.PutReturnTo(req.Context(), session, req.URL.RequestURI())
					if htmx.IsHTMX(req) {
						res.Header().Add(htmx.HeaderRedirect, "/login")
						res.WriteHeader(http.StatusUnauthorized)
						return
					}
					http.Redirect(res, req, "/login", http.StatusFound)
					return
				}

				// Rotate tokens in session.
				auth.SaveTokens(req.Context(), session, token)
				slogctx.Debug(req.Context(), "Token refresh successful.")
			}

			profile, ok := session.Get(req.Context(), "profile").(auth0.UserProfile)
			if !ok {
				slogctx.Warn(req.Context(), "Unable to retrieve profile from session.")
				auth.ClearAuth(req.Context(), session)
				auth.PutReturnTo(req.Context(), session, req.URL.RequestURI())
				if htmx.IsHTMX(req) {
					res.Header().Add(htmx.HeaderRedirect, "/login")
					res.WriteHeader(http.StatusUnauthorized)
					return
				}
				http.Redirect(res, req, "/login", http.StatusFound)
				return
			}

			if profile.Blocked {
				slogctx.Error(req.Context(), "Attempted access from blocked user. Redirecting to account issue page.",
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
			user, err := users.GetUserByExternalID(req.Context(), profile.GetID())
			if err != nil {
				slogctx.Error(req.Context(), "Get local user data failed.",
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
}

// RequireValidUser will ensure that protected routes have a valid user status before continuing.
func RequireValidUser(next http.Handler) http.Handler {
	paddleHandler := validatePaddleSubscription(next)
	androidHandler := validateAndroidSubscription(next)

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			res.WriteHeader(http.StatusForbidden)
			return
		}

		switch {
		case user.Metadata.Blocked:
			// User is blocked. Do not continue.
			slogctx.Error(req.Context(), "Blocked user.")
			res.WriteHeader(http.StatusForbidden)
			return

		case !user.Metadata.PoliciesAccepted:
			// User has not accepted policies, redirect to page asking them to contact support.
			slogctx.Error(req.Context(), "User has not accepted policies.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case user.InTrial():
			// User in trial, pass to next handler.
			next.ServeHTTP(res, req)
			return

		case !user.InTrial() && user.HasValidSubscription():
			// User not in trial and has a subscription. Pass to subscription validator handler.
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
			next.ServeHTTP(res, req)
			return

		default:
			// Unknown or unhandled user. Display a warning to contact support.
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return
		}
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
			slogctx.Error(req.Context(), "Get user paddle subscription failed.",
				slog.Any("error", err),
			)
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return
		}
		// NOTE: that trials are implemented outside of Paddle so we don't check the subscription status is trial here.
		switch {
		case paddle.IsPastDue(&userSubscription):
			// User account is past due or has other payment issues.
			slogctx.Error(req.Context(), "User account is past due.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case paddle.IsPaused(&userSubscription):
			// User subscription is paused.
			slogctx.Error(req.Context(), "User account is paused.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case paddle.IsCancelled(&userSubscription):
			// User subscription is canceled.
			slogctx.Error(req.Context(), "User has cancelled account.")
			http.Redirect(res, req, "/account-issue", http.StatusSeeOther)
			return

		case !paddle.IsActive(&userSubscription):
			// User subscription is not active.
			slogctx.Error(req.Context(), "User account requires activation.")
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
