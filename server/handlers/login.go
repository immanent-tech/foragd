// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/scheduler"
	"github.com/immanent-tech/foragd/scheduler/jobs"
	"github.com/immanent-tech/foragd/server/session"
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

	var user *models.User
	user, err = models.GetUserByExternalID(req.Context(), profile.GetID())
	switch {
	case err != nil && models.HTTPStatus(err) == http.StatusNotFound: // No local user.
		// Create a new local account for the user
		user, err = createNewLocalUser(req.Context(), *profile)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("create user: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
	case err != nil: // Backend error.
		HandleExternalError(&models.APIError{
			InternalError: fmt.Errorf("get user: %w", err),
			StatusCode:    http.StatusForbidden,
		}).ServeHTTP(res, req)
		return
	default: // Existing user.
		// Sync user data from the backend.
		auth0.SyncUser(req.Context(), user)
		if !user.Metadata.PoliciesAccepted {
			// User has not accepted policies, redirect to page asking them to contact support.
			slogctx.FromCtx(req.Context()).Error("User has not accepted policies.",
				slog.String("user_id", user.GetID()),
				slog.Any("error", err),
			)
			http.Redirect(res, req, models.RouteUserAccountIssue, http.StatusSeeOther)
			return
		}
	}
	ctx := models.UserToCtx(req.Context(), user)
	// Redirect the user appropriately.
	// ! Uncomment after beta.
	// if user.Metadata.Plan == "" {
	// 	// New user or user without a plan; redirect to choose subscription plan.
	// 	http.Redirect(res, req.WithContext(ctx), models.RouteCheckoutChoosePlan, http.StatusSeeOther)
	// 	return
	// }
	// if err := user.Metadata.Valid(); err != nil {
	// 	// User metadata is invalid, redirect user to page indicating they need to contact support to resolve the issue.
	// 	slogctx.FromCtx(req.Context()).Error("User data is invalid.",
	// 		slog.String("user_id", user.GetID()),
	// 		slog.Any("error", err),
	// 	)
	// 	http.Redirect(res, req.WithContext(ctx), models.RouteUserAccountIssue, http.StatusSeeOther)
	// 	return
	// }
	if !user.Active() {
		slogctx.FromCtx(req.Context()).Error("User is not active.",
			slog.String("user_id", user.GetID()),
			slog.Any("error", err),
		)
		// Account issues; redirect user to page indicating they need to contact support to resolve an issue with their account.
		http.Redirect(res, req.WithContext(ctx), models.RouteUserAccountIssue, http.StatusSeeOther)
		return
	}
	// ! Uncomment after beta.
	// if cancelled, endAt := user.Cancelled(); cancelled && endAt.Before(time.Now().UTC()) {
	// 	slogctx.FromCtx(req.Context()).Error("User has cancelled plan.",
	// 		slog.String("user_id", user.GetID()),
	// 		slog.Any("error", err),
	// 	)
	// 	// Account has been cancelled and past cancellation date; redirect to home page.
	// 	http.Redirect(res, req.WithContext(ctx), "/", http.StatusSeeOther)
	// 	return
	// }
	// Active user; redirect to home page.
	slogctx.FromCtx(ctx).Info("User logged in.",
		slog.String("user_id", user.GetID()),
	)
	auth0.ClearState(req)

	if returnTo, err := auth0.GetReturnTo(req); err != nil {
		http.Redirect(res, req.WithContext(ctx), "/home", http.StatusFound)
	} else {
		slogctx.FromCtx(ctx).Debug("Returning to previous page.",
			slog.String("return_to", returnTo),
		)
		http.Redirect(res, req.WithContext(ctx), returnTo, http.StatusFound)
	}
}

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
			ref = "/home"
		}
		if u, err := url.Parse(ref); err != nil || (u.Host != "" && u.Host != req.Host) {
			ref = "/home"
		}
		http.Redirect(res, req, ref, http.StatusFound)
	}
}

func createNewLocalUser(ctx context.Context, profile auth0.UserProfile) (*models.User, error) {
	// Create user object.
	user, err := auth0.CreateUserFromProfileData(ctx, &profile)
	if err != nil {
		return nil, fmt.Errorf("create user from profile: %w", err)
	}

	// Create and send a welcome email.
	email, err := resend.NewTemplatedEmail(
		"new-user",
		resend.To(user.GetEmail()),
		resend.WithTag(resend.TagCategory, resend.TagCategoryAccount),
		resend.WithTag(resend.TagUserID, user.GetID()),
		resend.WithVariable("USER_NICKNAME", user.GetNickname()),
		resend.WithVariable("USER_EMAIL", user.GetEmail()),
		resend.WithVariable("USER_AVATAR_URL", user.GetAvatar()),
	)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to create welcome email.",
			slog.String("user_id", user.GetID()),
			slog.Any("error", err),
		)
	}
	if err := resend.SendEmail(ctx, resend.WithExistingEmail(email)); err != nil {
		slogctx.FromCtx(ctx).Warn("Unable to send welcome email.",
			slog.String("user_id", user.GetID()),
			slog.Any("error", err),
		)
	}

	// Load the scheduler (but don't start it).
	if err := scheduler.LoadManager(ctx); err != nil {
		slogctx.FromCtx(ctx).Warn("Could not load scheduler, cannot schedule new user jobs.",
			slog.Any("error", err),
		)
	} else {
		// Create a new job, scheduled to run in ~2 days, that checks if the user has logged in yet, and sends them a
		// ping email if they haven't.
		for tip := range jobs.UserTipsJobs {
			job, err := jobs.NewUserTipsJob(user.GetID(), tip)
			if err != nil {
				slogctx.FromCtx(ctx).Warn("Could not create user tips job.",
					slog.String("tip", tip),
					slog.Any("error", err),
				)
			}
			if err := scheduler.Manager.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
				slogctx.FromCtx(ctx).Warn("Unable to schedule user tip job.",
					slog.String("tip", tip),
					slog.Any("error", err),
				)
			}
		}
	}

	return user, nil
}
