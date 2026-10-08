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
	auth handlers.Authenticator,
	session handlers.SessionManager,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			log := slogctx.FromCtx(req.Context())

			// Ignore updates route.
			if SkipUpdatesRoute(next)(res, req) {
				return
			}

			if handleUnauthenticated(log, auth, req, res) {
				return
			}

			if handleTokenExpiry(log, auth, req, res) {
				return
			}

			// Extract and validate user profile from session.
			externalUserID, blocked := extractUserProfile(session, req, log, auth, res)
			if externalUserID == "" {
				return
			}

			if blocked {
				handleBlockedUser(log, res, req, "/account-issue", externalUserID)
				return
			}

			// Fetch the user from the user management API.
			user, err := fetchUserFromAPI(log, users, req, externalUserID)
			if err != nil {
				return
			}

			// Add context values.
			ctx := enrichContext(req, user)

			// Pass to next request.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SkipUpdatesRoute skips the ExtractUserFromSession middleware for /updates routes.
func SkipUpdatesRoute(next http.Handler) func(res http.ResponseWriter, req *http.Request) bool {
	return func(res http.ResponseWriter, req *http.Request) bool {
		if strings.HasPrefix(req.URL.Path, "/updates") {
			res.WriteHeader(http.StatusOK)
			return true
		}
		res.WriteHeader(http.StatusContinue)
		return false
	}
}

// handleUnauthenticated handles redirecting unauthenticated users.
func handleUnauthenticated(
	log *slog.Logger,
	auth handlers.Authenticator,
	req *http.Request,
	res http.ResponseWriter,
) bool {
	if !auth.IsAuthenticated(req.Context()) {
		log.Warn("Unauthenticated; redirecting to login.")
		auth.PutReturnTo(req.Context(), req.URL.RequestURI())
		if htmx.IsHTMX(req) {
			res.Header().Add(htmx.HeaderRedirect, "/login")
			res.WriteHeader(http.StatusUnauthorized)
		} else {
			http.Redirect(res, req, "/login", http.StatusFound)
		}
		return true
	}
	return false
}

// handleTokenExpiry handles token refresh and redirect on failure.
func handleTokenExpiry(
	log *slog.Logger,
	auth handlers.Authenticator,
	req *http.Request,
	res http.ResponseWriter,
) bool {
	if auth.IsAccessTokenExpired(req.Context()) {
		refreshToken, err := auth.GetRefreshToken(req.Context())
		if err != nil || refreshToken == "" {
			log.Warn("Access token expired and no refresh token; redirecting to login.",
				slog.Any("error", err),
			)
			auth.ClearAuth(req.Context())
			auth.PutReturnTo(req.Context(), req.URL.RequestURI())
			if htmx.IsHTMX(req) {
				res.Header().Add(htmx.HeaderRedirect, "/login")
				res.WriteHeader(http.StatusUnauthorized)
			} else {
				http.Redirect(res, req, "/login", http.StatusFound)
			}
			return true
		}

		slogctx.Debug(req.Context(), "Access token expired; attempting refresh.")
		if err := auth.RefreshTokens(req.Context(), refreshToken); err != nil {
			log.Warn("Token refresh failed.",
				slog.Any("error", err),
			)
			auth.ClearAuth(req.Context())
			auth.PutReturnTo(req.Context(), req.URL.RequestURI())
			if htmx.IsHTMX(req) {
				res.Header().Add(htmx.HeaderRedirect, "/login")
				res.WriteHeader(http.StatusUnauthorized)
			} else {
				http.Redirect(res, req, "/login", http.StatusFound)
			}
			return true
		}

		// Rotate tokens in session.
		log.Debug("Token refresh successful.")
		return false
	}
	return false
}

// extractUserProfile extracts the external user ID and blocked status from the session profile.
func extractUserProfile(
	session handlers.SessionManager,
	req *http.Request,
	log *slog.Logger,
	auth handlers.Authenticator,
	res http.ResponseWriter,
) (string, bool) {
	var externalUserID string
	var blocked bool
	switch profile := session.Get(req.Context(), "profile").(type) {
	case auth0.UserProfile:
		// TODO: remove this block after a while.
		externalUserID = profile.GetID()
		blocked = profile.Blocked
	case models.UserProfileResponse:
		externalUserID = profile.GetID()
		if profile.Blocked != nil {
			blocked = *profile.Blocked
		} else {
			blocked = false
		}
	default:
		log.Warn("Unable to retrieve profile from session.")
		auth.ClearAuth(req.Context())
		auth.PutReturnTo(req.Context(), req.URL.RequestURI())
		if htmx.IsHTMX(req) {
			res.Header().Add(htmx.HeaderRedirect, "/login")
			res.WriteHeader(http.StatusUnauthorized)
		} else {
			http.Redirect(res, req, "/login", http.StatusFound)
		}
		return "", false
	}
	return externalUserID, blocked
}

// handleBlockedUser handles redirecting blocked users.
func handleBlockedUser(
	log *slog.Logger,
	res http.ResponseWriter,
	req *http.Request,
	redirectPath string,
	externalUserID string,
) {
	log.Error("Attempted access from blocked user. Redirecting to account issue page.",
		slog.String("external_user_id", externalUserID),
	)
	if htmx.IsHTMX(req) {
		res.Header().Set(htmx.HeaderRedirect, redirectPath)
	} else {
		http.Redirect(res, req, redirectPath, http.StatusTemporaryRedirect)
	}
}

// fetchUserFromAPI fetches and validates the user from the backend API.
func fetchUserFromAPI(
	log *slog.Logger,
	users UserService,
	req *http.Request,
	externalUserID string,
) (*models.User, error) {
	user, err := users.GetUserByExternalID(req.Context(), externalUserID)
	if err != nil {
		log.Error("Get local user data failed.",
			slog.String("external_user_id", externalUserID),
			slog.Any("error", err))
		return nil, err
	}
	return user, nil
}

// enrichContext adds user data to the request context.
func enrichContext(req *http.Request, user *models.User) context.Context {
	ctx := models.UserToCtx(req.Context(), user)
	ctx = slogctx.With(ctx, slog.String("user_id", user.GetID()))
	return ctx
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
			// User has cancelled account.
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
