/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package middlewares

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/paddle"
)

type UserService interface {
	GetUserByExternalID(ctx context.Context, externalID string) (*models.User, error)
}

type SessionManager interface {
	Get(ctx context.Context, key string) any
	Put(ctx context.Context, key string, value any)
	Remove(ctx context.Context, key string)
	RenewToken(ctx context.Context) error
	Clear(ctx context.Context) error
}

type Authenticator interface {
	IsAuthenticated(ctx context.Context) bool
	GenerateAuthURL(signup bool) (*auth0.AuthURLResult, error)
	StoreAuthRequest(ctx context.Context, r *auth0.AuthURLResult)
	ValidateState(ctx context.Context, got string) error
	PerformExchange(ctx context.Context, code string) (*models.UserProfileResponse, error)
	EnsureFresh(ctx context.Context) error
	PutReturnTo(ctx context.Context, path string)
	ConsumeReturnTo(ctx context.Context) (string, error)
	GenerateLogoutURL() (*url.URL, error)
	ClearAuth(ctx context.Context)
}

// ExtractUserFromSession will make sure the session has a usable access token (refreshing it if needed), extract the
// user data from the session, retrieve the user details from the backend and then store the user object in the context
// for use by later handlers.
//
// NOTE: the old SkipUpdatesRoute step was removed. It never called next (so matching routes got an empty 200) and it
// wrote a "100 Continue" header on every other request. If some routes must bypass authentication, mount them outside
// the router group that uses this middleware.
func ExtractUserFromSession(
	users UserService,
	auth Authenticator,
	session SessionManager,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			log := slogctx.FromCtx(req.Context())

			// Make sure there is a valid (or refreshable) session. This also covers the "not authenticated" case.
			if !ensureAuthenticated(log, auth, res, req) {
				return
			}

			// Extract and validate user profile from session.
			externalUserID, blocked, ok := extractUserProfile(session, req, log, auth, res)
			if !ok {
				return
			}

			if blocked {
				handleBlockedUser(log, res, req, "/account-issue", externalUserID)
				return
			}

			// Fetch the user from the user management API.
			user, err := fetchUserFromAPI(log, users, req, externalUserID)
			if err != nil {
				handleUserLookupError(auth, res, req, err)
				return
			}

			// Add context values.
			ctx := enrichContext(req, user)

			// Pass to next request.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// redirectToLogin sends the client to the login page. htmx requests get an HX-Redirect so the whole page navigates.
func redirectToLogin(res http.ResponseWriter, req *http.Request) {
	if htmx.IsHTMX(req) {
		res.Header().Set(htmx.HeaderRedirect, "/login")
		res.WriteHeader(http.StatusUnauthorized)
		return
	}
	http.Redirect(res, req, "/login", http.StatusFound)
}

// ensureAuthenticated makes sure the session holds a usable access token, refreshing it if necessary. It returns true
// if the request may continue. Otherwise a response has already been written and the caller must stop.
func ensureAuthenticated(
	log *slog.Logger,
	auth Authenticator,
	res http.ResponseWriter,
	req *http.Request,
) bool {
	err := auth.EnsureFresh(req.Context())
	switch {
	case err == nil:
		return true

	case errors.Is(err, auth0.ErrSessionExpired):
		// Not logged in, or the refresh token is gone/revoked. The user has to log in again.
		log.Warn("Not authenticated or session expired; redirecting to login.",
			slog.Any("error", err),
		)
		auth.PutReturnTo(req.Context(), req.URL.RequestURI())
		redirectToLogin(res, req)

	default:
		// Transient failure talking to the auth provider (network, 5xx). The session is intact, so do NOT log the
		// user out: tell the client to retry.
		log.Error("Could not refresh access token.",
			slog.Any("error", err),
		)
		res.Header().Set("Retry-After", "5")
		http.Error(res, "authentication service unavailable", http.StatusServiceUnavailable)
	}
	return false
}

// extractUserProfile extracts the external user ID and blocked status from the session profile. ok is false if a
// response has already been written (the profile was missing or invalid) and the caller must stop.
func extractUserProfile(
	session SessionManager,
	req *http.Request,
	log *slog.Logger,
	auth Authenticator,
	res http.ResponseWriter,
) (externalUserID string, blocked, ok bool) {
	switch profile := session.Get(req.Context(), "profile").(type) {
	case auth0.UserProfile:
		// TODO: remove this block after a while.
		externalUserID = profile.GetID()
		blocked = profile.Blocked
	case models.UserProfileResponse:
		externalUserID = profile.GetID()
		blocked = profile.Blocked != nil && *profile.Blocked
	case *models.UserProfileResponse:
		if profile != nil {
			externalUserID = profile.GetID()
			blocked = profile.Blocked != nil && *profile.Blocked
		}
	}

	if externalUserID == "" {
		log.Warn("Unable to retrieve profile from session.")
		auth.ClearAuth(req.Context())
		auth.PutReturnTo(req.Context(), req.URL.RequestURI())
		redirectToLogin(res, req)
		return "", false, false
	}
	return externalUserID, blocked, true
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

// handleUserLookupError writes a response for a failed local user lookup, so the request never ends with an empty 200.
func handleUserLookupError(auth Authenticator, res http.ResponseWriter, req *http.Request, err error) {
	if models.HTTPStatus(err) == http.StatusNotFound {
		// The session refers to a user we no longer have locally. Force a fresh login: the login callback recreates
		// the local account from the auth provider's profile.
		auth.ClearAuth(req.Context())
		redirectToLogin(res, req)
		return
	}
	res.Header().Set("Retry-After", "5")
	http.Error(res, "unable to load user", http.StatusServiceUnavailable)
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
			if htmx.IsHTMX(req) {
				res.Header().Set(htmx.HeaderRedirect, "/account-issue")
			} else {
				http.Redirect(res, req, "/account-issue", http.StatusTemporaryRedirect)
			}
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
