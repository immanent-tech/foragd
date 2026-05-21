// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/scheduler"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
)

type Login struct {
	template templ.Component
}

func (p *Login) FullResponse(w http.ResponseWriter, r *http.Request) {
	templ.Handler(p.template).ServeHTTP(w, r)
}

func (p *Login) PartialResponse(w http.ResponseWriter, r *http.Request) {
	templ.Handler(p.template, templ.WithFragments(templates.BodyFragment)).ServeHTTP(w, r)
}

// HandleLogin handles user login or sign-up requests.
func HandleLogin(res http.ResponseWriter, req *http.Request) {
	// Redirect to home if already authenticated.
	if auth0.IsAuthenticated(req) {
		http.Redirect(res, req, "/home", http.StatusFound)
		return
	}

	// Generate state, verification and authentication URL.
	result, err := auth0.GenerateAuthURL(req)
	if err != nil {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("generate auth url: %w", err),
			StatusCode:    http.StatusInternalServerError,
		}).ServeHTTP(res, req)
		return
	}

	// Renew session token before writing to prevent session fixation.
	if err := session.Renew(req.Context()); err != nil {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("renew session token: %w", err),
			StatusCode:    http.StatusInternalServerError,
		}).ServeHTTP(res, req)
		return
	}

	// Store data required for verification.
	auth0.PutState(req, result.State)
	auth0.PutCodeVerifier(req, result.CodeVerifier)

	// Redirect for authentication.
	slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.",
		slog.String("url", result.URL),
	)
	http.Redirect(res, req, result.URL, http.StatusFound)
}

// HandleLoginCallback handles processing the response from the login provider.
func HandleLoginCallback(res http.ResponseWriter, req *http.Request) {
	// Check for errors returned by Auth0.
	if errCode := req.FormValue("error"); errCode != "" {
		errDesc := req.FormValue("error_description")
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("auth0 returned an error: %s: %s", errCode, errDesc),
			StatusCode:    http.StatusBadRequest,
		}).ServeHTTP(res, req)
		return
	}

	// Validate state to prevent CSRF.
	if state, err := auth0.GetState(req); err != nil || req.FormValue("state") != state {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("restore state: %w", err),
			StatusCode:    http.StatusBadRequest,
		}).ServeHTTP(res, req)
		return
	}

	// Exchange an authorization code for a token.
	code := req.FormValue("code")
	verifier, err := auth0.GetCodeVerifier(req)
	if err != nil {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("restore verifier: %w", err),
			StatusCode:    http.StatusBadRequest,
		}).ServeHTTP(res, req)
		return
	}

	token, profile, err := auth0.Exchange(req.Context(), code, verifier)
	if err != nil {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("exchange auth token: %w", err),
			StatusCode:    http.StatusBadRequest,
		}).ServeHTTP(res, req)
		return
	}

	// Renew session token before writing to prevent session fixation.
	if err := session.Renew(req.Context()); err != nil {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("renew session token: %w", err),
			StatusCode:    http.StatusInternalServerError,
		}).ServeHTTP(res, req)
		return
	}

	// Save tokens to session.
	auth0.SaveTokens(req.Context(), token)
	// Save profile to session.
	session.Save(req.Context(), "profile", profile)

	// loginChain := alice.New()

	var user *models.User
	user, err = service.GetUserByExternalID(req.Context(), profile.GetID())
	switch {
	case err != nil && models.HTTPStatus(err) != http.StatusNotFound: // Backend error.
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("get user: %w", err),
			StatusCode:    http.StatusForbidden,
		}).ServeHTTP(res, req)
		return
	case err != nil && models.HTTPStatus(err) == http.StatusNotFound: // No local user.
		// Create a new local account for the user
		// subscription_plan, err := session.Restore[string](req.Context(), "subscription_plan")
		// if err != nil {
		// 	subscription_plan = "annual"
		// }
		newUser, err := auth0.CreateUserFromProfileData(req.Context(), profile)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("create user from profile: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		user = newUser

		// Create and send a welcome email.
		email, err := resend.NewTemplatedEmail(
			"new-user",
			resend.WithTo(user.GetEmail()),
			resend.WithTag(resend.TagCategory, resend.TagCategoryAccount),
			resend.WithTag(resend.TagUserID, user.GetID()),
			resend.WithVariable("USER_NICKNAME", user.GetNickname()),
			resend.WithVariable("USER_EMAIL", user.GetEmail()),
			resend.WithVariable("USER_AVATAR_URL", user.GetAvatar()),
		)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("create welcome email: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		if err := resend.SendEmail(req.Context(), resend.WithExistingEmail(email)); err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("send welcome email: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		// Load the scheduler (but don't start it).
		if err := scheduler.LoadManager(req.Context()); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Could not load scheduler, cannot schedule new user jobs.",
				slog.Any("error", err),
			)
		} else {
			for email := range slices.Values([]models.UserTipsEmail{models.UserTipsEmailNewInactiveUser, models.UserTipsEmailTipEmailNewsletters}) {
				job, err := jobs.NewUserTipsJob(user.GetID(), email)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not create user tips job.",
						slog.String("tip", string(email)),
						slog.Any("error", err),
					)
				}
				if err := scheduler.Manager.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to schedule user tip job.",
						slog.String("tip", string(email)),
						slog.Any("error", err),
					)
				}
			}
		}
	default: // Existing user.
		// Sync user data from the backend.
		service.SyncUser(req.Context(), user)
	}

	ctx := models.UserToCtx(req.Context(), user)

	// loginChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
	slogctx.FromCtx(ctx).Info("User logged in.",
		slog.String("user_id", user.GetID()),
	)
	auth0.ClearState(req)

	if returnTo, err := auth0.GetReturnTo(req); err != nil {
		slogctx.FromCtx(ctx).Debug("Redirecting home.")
		http.Redirect(res, req.WithContext(ctx), "/home", http.StatusFound)
	} else {
		slogctx.FromCtx(ctx).Debug("Returning to previous page.",
			slog.String("return_to", returnTo),
		)
		http.Redirect(res, req.WithContext(ctx), returnTo, http.StatusFound)
	}
	// }).ServeHTTP(res, req)
}

// func handleNewUser(h http.Handler) http.Handler {
// 	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
// 		// Create a new local account for the user
// 		user, err = createNewLocalUser(req.Context(), *profile)
// 		if err != nil {
// 			HandleExternalError(&models.APIError{
// 				InternalError: fmt.Errorf("create user: %w", err),
// 				StatusCode:    http.StatusInternalServerError,
// 			}).ServeHTTP(res, req)
// 			return
// 		}

// 	})
// }

// HandleLoginError handles login errors, including invalid login callback URL, missing parameters, expired password
// reset links.
func HandleLoginError(res http.ResponseWriter, req *http.Request) {
	slogctx.FromCtx(req.Context()).Error("Auth0 reported a login error.",
		slog.String("client_id", req.URL.Query().Get("client_id")),
		slog.String("error_code", req.URL.Query().Get("error")),
		slog.String("error_description", req.URL.Query().Get("error_description")),
		slog.String("tracking", req.URL.Query().Get("tracking")),
	)
	HandleExternalError(&models.APIError{
		InternalError: errors.New("login failed"),
		StatusCode:    http.StatusForbidden,
	}).ServeHTTP(res, req)
}

// HandleRefreshToken handles refreshing the user's access token (using a refresh token) when it is about to expire.
func HandleRefreshToken(res http.ResponseWriter, req *http.Request) {
	// Retrieve the refresh token and expiry from the session.
	tkn, err := auth0.GetRefreshToken(req)
	if err != nil {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("get refresh token from session: %w", err),
			StatusCode:    http.StatusBadRequest,
		}).ServeHTTP(res, req)
		return
	}
	expiry, err := auth0.GetTokenExpiry(req)
	if err != nil {
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("get token expiry from session: %w", err),
			StatusCode:    http.StatusBadRequest,
		}).ServeHTTP(res, req)
		return
	}

	// If token will expire soon, refresh it.
	const refreshGracePeriod = time.Hour
	if expiry.UTC().Sub(time.Now().UTC()) < refreshGracePeriod {
		token, err := auth0.RefreshTokens(req.Context(), tkn)
		if err != nil {
			auth0.ClearAuth(req)
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Save tokens to session.
		auth0.SaveTokens(req.Context(), token)

		// Redirect back to the referrer or home (same-origin only).
		ref := req.Referer()
		if ref == "" {
			ref = RouteHome
		}
		if u, err := url.Parse(ref); err != nil || (u.Host != "" && u.Host != req.Host) {
			ref = RouteHome
		}
		http.Redirect(res, req, ref, http.StatusFound)
	}
}
