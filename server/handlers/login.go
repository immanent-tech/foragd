/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
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

		planID := req.URL.Query().Get("subscription_plan")
		m.SessionMgr.Put(req.Context(), "subscription_plan", planID)

		// Redirect the user appropriately.
		var signup bool
		route := chi.RouteContext(req.Context()).RoutePattern()
		switch {
		case strings.HasSuffix(route, "/signup"):
			signup = true
		case strings.HasSuffix(route, "/login"):
			signup = false
		}
		result, err := authenticator.GenerateAuthURL(signup)
		if err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("generate auth url: %w", err),
				StatusCode:    http.StatusInternalServerError,
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

		// Store data required for verification.
		authenticator.StoreAuthRequest(req.Context(), result)

		// Redirect for authentication.
		slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.")
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
		if err := m.processLoginCallback(res, req, userSvc, authMgr, auth, emailSender); err != nil {
			m.HandleExternalError(err).ServeHTTP(res, req)
		}
	}
}

// processLoginCallback processes the login callback and returns an error if something goes wrong.
// This separates HTTP concerns from business logic to make unit testing easier.
func (m *Manager) processLoginCallback(
	res http.ResponseWriter,
	req *http.Request,
	userSvc UserService,
	authMgr AuthManager,
	auth Authenticator,
	emailSender EmailSender,
) error {
	ctx := req.Context()
	log := slogctx.FromCtx(ctx)

	// Check for errors returned by Auth0.
	if errCode := req.FormValue("error"); errCode != "" {
		errDesc := req.FormValue("error_description")
		return fmt.Errorf("auth0 returned an error: %s: %s", errCode, errDesc)
	}

	// Validate state value.
	if err := auth.ValidateState(ctx, req.FormValue("state")); err != nil {
		return fmt.Errorf("restore state: %w", err)
	}

	// Verify and exchange authorization code to retrieve user profile.
	profile, err := auth.PerformExchange(ctx, req.FormValue("code"))
	if err != nil {
		return fmt.Errorf("exchange auth token: %w", err)
	}

	// Save profile to session.
	m.SessionMgr.Put(ctx, "profile", profile)

	// Retrieve the local user associated with the profile.
	var user *models.User
	var errCreate error
	user, errCreate = userSvc.GetUserByExternalID(ctx, profile.GetID())
	switch {
	case errCreate != nil && models.HTTPStatus(errCreate) != http.StatusNotFound: // Backend error.
		return fmt.Errorf("get user: %w", errCreate)
	case errCreate != nil && models.HTTPStatus(errCreate) == http.StatusNotFound: // No local user.
		// Create a new local account for the user
		user, errCreate = createUserFromProfileData(ctx, userSvc, authMgr, profile)
		if errCreate != nil {
			return fmt.Errorf("create user from profile: %w", errCreate)
		}
		if err := m.sendNewUserEmails(ctx, emailSender, user); err != nil {
			log.Warn("Could not send welcome emails.", slog.Any("error", err))
		}

	default: // Existing user.
		// Sync user data from the backend.
		if err := m.syncUser(ctx, userSvc, authMgr, user); err != nil {
			return fmt.Errorf("sync user: %w", err)
		}
	}

	m.redirectAfterLogin(res, req, auth, user)
	return nil
}

func (m *Manager) sendNewUserEmails(ctx context.Context, emailSender EmailSender, user *models.User) error {
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
	// Schedule follow-up email jobs.
	return m.scheduleNewUserEmailJobs(ctx, user)
}

// scheduleNewUserEmailJobs schedules follow-up email jobs for new users.
// This separates the scheduling concern from email sending to make testing easier.
func (m *Manager) scheduleNewUserEmailJobs(ctx context.Context, user *models.User) error {
	manager, err := scheduler.New()
	if err != nil {
		log := slogctx.FromCtx(ctx)
		log.Warn("Could not load scheduler, cannot schedule new user jobs.",
			slog.Any("error", err))
		return nil
	}

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
			log := slogctx.FromCtx(ctx)
			log.Warn("Could not create user tips job.",
				slog.String("email", id),
				slog.Any("error", err))
			continue
		}
		if err := manager.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
			log := slogctx.FromCtx(ctx)
			log.Warn("Unable to schedule user tip job.",
				slog.String("email", id),
				slog.Any("error", err))
			continue
		}
		log := slogctx.FromCtx(ctx)
		log.Info("Scheduled user email.",
			slog.String("email", id),
			slog.Time("scheduled_at", time.Now().UTC().Add(delay)))
	}

	return nil
}

// redirectAfterLogin redirects the user after a successful login.
func (m *Manager) redirectAfterLogin(
	res http.ResponseWriter,
	req *http.Request,
	auth Authenticator,
	user *models.User,
) {
	log := slogctx.FromCtx(req.Context())
	ctx := models.UserToCtx(req.Context(), user)

	log.Info("User logged in.", slog.String("user_id", user.GetID()))

	if returnTo, err := auth.ConsumeReturnTo(req.Context()); err != nil {
		log.Debug("Redirecting home.")
		http.Redirect(res, req.WithContext(ctx), "/home", http.StatusFound)
	} else {
		log.Debug("Returning to previous page.", slog.String("return_to", returnTo))
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
func (m *Manager) HandleRefreshToken(auth Authenticator) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		err := auth.EnsureFresh(req.Context())
		switch {
		case err == nil:
			res.WriteHeader(http.StatusNoContent)
		case errors.Is(err, auth0.ErrSessionExpired):
			res.Header().Set("HX-Redirect", "/login")
			res.WriteHeader(http.StatusUnauthorized)
		default:
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("refresh tokens: %w", err),
				StatusCode:    http.StatusServiceUnavailable,
			}).ServeHTTP(res, req)
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
	if loginCount := auth0User.GetUserResponseContent.LoginsCount; loginCount != nil {
		user.LoginCount = *loginCount
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
func (m *Manager) syncUser(ctx context.Context, userSvc UserService, authMgr AuthManager, user *models.User) error {
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
		return err
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
			return err
		}
	}
	return nil
}
