/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"go.opentelemetry.io/otel/codes"

	"github.com/immanent-tech/go-base/server/forms"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	gerror "github.com/immanent-tech/foragd/providers/google/error"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/scheduler"
	"github.com/immanent-tech/foragd/scheduler/jobs"
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
func (m *Manager) HandleLogin(authenticator Authenticator) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Redirect to home if already authenticated.
		if authenticator.IsAuthenticated(req.Context()) {
			http.Redirect(res, req, "/home", http.StatusFound)
			return
		}

		// Generate state, verification and authentication URL.
		result, err := authenticator.GenerateAuthURL(req)
		if err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("generate auth url: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		planID := req.URL.Query().Get("subscription_plan")
		m.SessionMgr.Put(req.Context(), "subscription_plan", planID)

		// Renew session token before writing to prevent session fixation.
		if err := m.SessionMgr.RenewToken(req.Context()); err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("renew m.SessionMgr token: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Store data required for verification.
		authenticator.PutState(req.Context(), result.GetState())
		authenticator.PutCodeVerifier(req.Context(), result.GetCodeVerifier())

		// Redirect for authentication.
		slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.",
			slog.String("url", result.GetURL()),
		)
		http.Redirect(res, req, result.GetURL(), http.StatusFound)
	}
}

// HandleLoginCallback handles processing the response from the login provider.
func (m *Manager) HandleLoginCallback(
	userSvc UserService,
	authMgr AuthManager,
	auth Authenticator,
	emailSender EmailSender,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		log := slogctx.FromCtx(req.Context())
		// Check for errors returned by Auth0.
		if errCode := req.FormValue("error"); errCode != "" {
			errDesc := req.FormValue("error_description")
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("auth0 returned an error: %s: %s", errCode, errDesc),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// Validate state to prevent CSRF.
		if state, err := auth.GetState(req.Context()); err != nil ||
			req.FormValue("state") != state {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("restore state: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// Exchange an authorization code for a token.
		code := req.FormValue("code")
		verifier, err := auth.GetCodeVerifier(req.Context())
		if err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("restore verifier: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// Verify and exchange authorization code to retrieve user profile.
		profile, err := auth.PerformExchange(req.Context(), code, verifier)
		if err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("exchange auth token: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// Renew session token before writing to prevent session fixation.
		if err := m.SessionMgr.RenewToken(req.Context()); err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("renew m.SessionMgr token: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Save profile to session.
		m.SessionMgr.Put(req.Context(), "profile", profile)

		// Retrieve the local user associated with the profile.
		var user *models.User
		user, err = userSvc.GetUserByExternalID(req.Context(), profile.GetID())
		switch {
		case err != nil && models.HTTPStatus(err) != http.StatusNotFound: // Backend error.
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get user: %w", err),
				StatusCode:    http.StatusForbidden,
			}).ServeHTTP(res, req)
			return
		case err != nil && models.HTTPStatus(err) == http.StatusNotFound: // No local user.
			// Create a new local account for the user
			newUser, err := createUserFromProfileData(req.Context(), userSvc, authMgr, profile)
			if err != nil {
				m.HandleExternalError(&models.APIError{
					InternalError: fmt.Errorf("create user from profile: %w", err),
					StatusCode:    http.StatusInternalServerError,
				}).ServeHTTP(res, req)
				return
			}
			user = newUser
			if err := sendNewUserEmails(req.Context(), emailSender, user); err != nil {
				log.Warn("Could not send welcome emails.",
					slog.Any("error", err))
			}

		default: // Existing user.
			// Sync user data from the backend.
			syncUser(req.Context(), userSvc, authMgr, user)
		}

		redirectAfterLogin(res, req, m.SessionMgr, auth, user)
	}
}

func sendNewUserEmails(ctx context.Context, emailSender EmailSender, user *models.User) error {
	log := slogctx.FromCtx(ctx)
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
		return fmt.Errorf("create welcome email: %w", err)
	}
	if err := emailSender.Send(ctx, resend.WithExistingEmail(email)); err != nil {
		return fmt.Errorf("send welcome email: %w", err)
	}
	// Load the scheduler (but don't start it).
	manager, err := scheduler.NewManager(ctx)
	if err != nil {
		log.Warn("Could not load scheduler, cannot schedule new user jobs.",
			slog.Any("error", err),
		)
	} else {
		var scheduledEmails = map[models.EmailTemplateID]time.Duration{
			// Send 1st tip: email newsletters after 1 day.
			"tip-email-newsletters": 24 * time.Hour,
			// Check and send inactive ping after 5 days if not active.
			"new-inactive-user": 5 * 24 * time.Hour,
			// Send check-in after 7 days.
			"trial-checkin": models.DefaultTrialPeriod - 15*24*time.Hour,
			// Send expiry reminder in two days of expiry.
			"trial-expiring": models.DefaultTrialPeriod - 48*time.Hour,
		}

		for id, delay := range scheduledEmails {
			job, err := jobs.NewUserEmailJob(user.GetID(), id, delay)
			if err != nil {
				log.Warn("Could not create user tips job.",
					slog.String("email", id),
					slog.Any("error", err),
				)
			}
			if err := manager.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
				log.Warn("Unable to schedule user tip job.",
					slog.String("email", id),
					slog.Any("error", err),
				)
			}
			log.Info("Scheduled user email.",
				slog.String("email", id),
				slog.Time("scheduled_at", time.Now().UTC().Add(delay)),
			)
		}
	}

	return nil
}

func redirectAfterLogin(
	res http.ResponseWriter,
	req *http.Request,
	session SessionManager,
	authenticator Authenticator,
	user *models.User,
) {
	log := slogctx.FromCtx(req.Context())
	ctx := models.UserToCtx(req.Context(), user)

	log.Info("User logged in.",
		slog.String("user_id", user.GetID()),
	)
	authenticator.ClearState(req.Context())

	if returnTo, err := authenticator.GetReturnTo(req.Context()); err != nil {
		log.Debug("Redirecting home.")
		http.Redirect(res, req.WithContext(ctx), "/home", http.StatusFound)
	} else {
		log.Debug("Returning to previous page.",
			slog.String("return_to", returnTo),
		)
		http.Redirect(res, req.WithContext(ctx), returnTo, http.StatusFound)
	}
}

// HandleLoginError handles login errors, including invalid login callback URL, missing parameters, expired password
// reset links.
func (m *Manager) HandleLoginError(res http.ResponseWriter, req *http.Request) {
	auth0Err, err := forms.DecodeForm[*auth0.Error](req)
	if err != nil {
		slogctx.Error(req.Context(), "Could not decode Auth0 error response.", slog.Any("error", err))
		gerror.ReportError(err)
	} else {
		gerror.ReportError(fmt.Errorf("%s: %s (tracking: %s)", auth0Err.Code, auth0Err.Message, auth0Err.Tracking))
		slogctx.FromCtx(req.Context()).Error("Auth0 reported a login error.",
			slog.String("client_id", auth0Err.ClientID),
			slog.String("error_code", auth0Err.Code),
			slog.String("error_description", auth0Err.Message),
			slog.String("tracking", auth0Err.Tracking),
		)
	}
	RenderExternalPage(&AccountIssue{svc: m.NewPageServices()}).ServeHTTP(res, req)
}

// HandleRefreshToken handles refreshing the user's access token (using a refresh token) when it is about to expire.
func (m *Manager) HandleRefreshToken(
	authenticator Authenticator,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the refresh token and expiry from the session.
		tkn, err := authenticator.GetRefreshToken(req.Context())
		if err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get refresh token from m.SessionMgr: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}
		expiry, err := authenticator.GetTokenExpiry(req.Context())
		if err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get token expiry from m.SessionMgr: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// If token will expire soon, refresh it.
		const refreshGracePeriod = time.Hour
		if expiry.UTC().Sub(time.Now().UTC()) < refreshGracePeriod {
			if err := authenticator.RefreshTokens(req.Context(), tkn); err != nil {
				authenticator.ClearAuth(req.Context())
				http.Redirect(res, req, "/login", http.StatusSeeOther)
				return
			}

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
}

// createUserFromProfileData creates a new user from the external provider details.
func createUserFromProfileData(
	ctx context.Context,
	userSvc UserService,
	authMgr AuthManager,
	profile *models.UserProfileResponse,
) (*models.User, error) {
	auth0User, err := authMgr.GetUser(ctx, profile.GetID())
	if err != nil {
		return nil, fmt.Errorf("get user details: %w", err)
	}

	id := "user_" + strconv.FormatUint(xxh3.Hash([]byte(profile.GetID())), 10)
	ts := time.Now().UTC()
	lastLogin := auth0User.GetUserResponseContent.GetLastLogin()
	if lastLogin.IsZero() {
		lastLogin = models.UnixEpoch
	}
	user := &models.User{
		CreatedAt:      ts,
		UpdatedAt:      &ts,
		ExternalUserID: profile.GetID(),
		Provider:       strings.Split(profile.GetID(), "|")[0],
		Email:          auth0User.GetUserResponseContent.GetEmail(),
		UserID:         id,
		AvatarURL:      new(auth0User.GetUserResponseContent.GetPicture()),
		LoginCount:     *auth0User.GetUserResponseContent.LoginsCount,
		LastLogin:      lastLogin,
		Metadata: models.UserMetadata{
			EmailVerified:    auth0User.GetUserResponseContent.GetEmailVerified(),
			PromotionalEmail: true,
		},
		Settings: models.UserSettings{
			ShowOnboarding:        true,
			ShowSubscriptionStats: false,
			MarkArticleReadOnView: true,
		},
	}
	if accepted, ok := auth0User.GetUserResponseContent.GetAppMetadata()["policies_accepted"].(bool); ok {
		user.Metadata.PoliciesAccepted = accepted
	}

	if err := userSvc.AddUser(ctx, user); err != nil {
		return nil, fmt.Errorf("add user: %w", err)
	}

	return user, nil
}

// syncUser tries to sync relevant user data from the auth backend to the local data.
func syncUser(ctx context.Context, userSvc UserService, authMgr AuthManager, user *models.User) {
	ctx, span := tracer.Start(ctx, "SyncUser")
	defer span.End()
	log := slogctx.FromCtx(ctx)

	auth0User, err := authMgr.GetUser(ctx, user.GetExternalID())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Error("Could not sync user data.",
			slog.String("user_id", user.GetID()),
			slog.Any("error", err))
		return
	}

	// Create needed updates by comparing request values to existing user values and adding new values to updates map as appropriate.
	updates := make(map[string]any)
	// Overwrite local avatar with remote avatar if different
	if avatarURL := auth0User.GetUserResponseContent.GetPicture(); user.GetAvatar() != avatarURL {
		updates["avatar_url"] = avatarURL
		user.AvatarURL = &avatarURL
	}
	// Overwrite local nickname with remote nickname if different
	if nickname := auth0User.GetUserResponseContent.GetNickname(); user.GetNickname() != nickname {
		updates["nickname"] = nickname
		user.Nickname = nickname
	}
	// Overwrite local email with remote email if different
	if email := auth0User.GetUserResponseContent.GetEmail(); user.GetEmail() != email {
		updates["email"] = email
		user.Email = email
	}
	// Update login count.
	updates["login_count"] = auth0User.GetUserResponseContent.GetLoginsCount()
	// Update last login timestamp.
	if lastLogin := auth0User.GetUserResponseContent.GetLastLogin(); lastLogin.After(user.LastLogin) {
		updates["last_login"] = lastLogin
	}

	// Update user metadata.
	metadata := user.Metadata
	if accepted, ok := auth0User.GetUserResponseContent.GetAppMetadata()["policies_accepted"].(bool); ok &&
		metadata.PoliciesAccepted != accepted {
		metadata.PoliciesAccepted = accepted
	}
	if emailVerified := auth0User.GetUserResponseContent.GetEmailVerified(); emailVerified != metadata.EmailVerified {
		metadata.EmailVerified = emailVerified
	}
	user.Metadata = metadata
	updates["metadata"] = metadata

	// If no updates are necessary, bail early.
	if len(updates) > 0 {
		if err := userSvc.UpdateUser(ctx, user, updates); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			slogctx.Error(ctx, "Could not sync user data.",
				slog.String("user_id", user.GetID()),
				slog.Any("error", err))
			return
		}
	}
}
