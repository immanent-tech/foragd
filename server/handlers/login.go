// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/a-h/templ"
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
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
func (m *Manager) HandleLogin(authenticator *auth0.Authenticator) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Redirect to home if already authenticated.
		if authenticator.IsAuthenticated(req.Context(), m.SessionMgr) {
			http.Redirect(res, req, "/home", http.StatusFound)
			return
		}

		// Generate state, verification and authentication URL.
		result, err := authenticator.GenerateAuthURL(req)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("generate auth url: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		planID := req.URL.Query().Get("subscription_plan")
		m.SessionMgr.Put(req.Context(), "subscription_plan", planID)

		// Renew m.SessionMgr token before writing to prevent m.SessionMgr fixation.
		if err := m.SessionMgr.RenewToken(req.Context()); err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("renew m.SessionMgr token: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Store data required for verification.
		authenticator.PutState(req.Context(), m.SessionMgr, result.GetState())
		authenticator.PutCodeVerifier(req.Context(), m.SessionMgr, result.GetCodeVerifier())

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
	authenticator *auth0.Authenticator,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
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
		if state, err := authenticator.GetState(
			req.Context(),
			m.SessionMgr,
		); err != nil ||
			req.FormValue("state") != state {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("restore state: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// Exchange an authorization code for a token.
		code := req.FormValue("code")
		verifier, err := authenticator.GetCodeVerifier(req.Context(), m.SessionMgr)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("restore verifier: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		token, profile, err := authenticator.PerformExchange(req.Context(), code, verifier)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("exchange auth token: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// Renew m.SessionMgr token before writing to prevent m.SessionMgr fixation.
		if err := m.SessionMgr.RenewToken(req.Context()); err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("renew m.SessionMgr token: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Save tokens to m.SessionMgr.
		authenticator.SaveTokens(req.Context(), m.SessionMgr, token)
		// Save profile to m.SessionMgr.
		m.SessionMgr.Put(req.Context(), "profile", profile)

		var user *models.User
		user, err = userSvc.GetUserByExternalID(req.Context(), profile.GetID())
		switch {
		case err != nil && models.HTTPStatus(err) != http.StatusNotFound: // Backend error.
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get user: %w", err),
				StatusCode:    http.StatusForbidden,
			}).ServeHTTP(res, req)
			return
		case err != nil && models.HTTPStatus(err) == http.StatusNotFound: // No local user.
			// Create a new local account for the user
			// subscription_plan, err := m.SessionMgr.Restore[string](req.Context(), "subscription_plan")
			// if err != nil {
			// 	subscription_plan = "annual"
			// }
			newUser, err := auth0.CreateUserFromProfileData(req.Context(), userSvc, profile)
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
			manager, err := scheduler.NewManager(req.Context())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Could not load scheduler, cannot schedule new user jobs.",
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
						slogctx.FromCtx(req.Context()).Warn("Could not create user tips job.",
							slog.String("email", id),
							slog.Any("error", err),
						)
					}
					if err := manager.ScheduleJob(job.JobDetail(), job.Trigger()); err != nil {
						slogctx.FromCtx(req.Context()).Warn("Unable to schedule user tip job.",
							slog.String("email", id),
							slog.Any("error", err),
						)
					}
					slogctx.FromCtx(req.Context()).Warn("Scheduled user email.",
						slog.String("email", id),
						slog.Time("scheduled_at", time.Now().UTC().Add(delay)),
					)
				}
			}
		default: // Existing user.
			// Sync user data from the backend.
			userSvc.SyncUser(res, req, user)
		}

		ctx := models.UserToCtx(req.Context(), user)

		slogctx.FromCtx(ctx).Info("User logged in.",
			slog.String("user_id", user.GetID()),
		)
		authenticator.ClearState(req.Context(), m.SessionMgr)

		if returnTo, err := authenticator.GetReturnTo(req.Context(), m.SessionMgr); err != nil {
			slogctx.FromCtx(ctx).Debug("Redirecting home.")
			http.Redirect(res, req.WithContext(ctx), "/home", http.StatusFound)
		} else {
			slogctx.FromCtx(ctx).Debug("Returning to previous page.",
				slog.String("return_to", returnTo),
			)
			http.Redirect(res, req.WithContext(ctx), returnTo, http.StatusFound)
		}
	}
}

// HandleLoginError handles login errors, including invalid login callback URL, missing parameters, expired password
// reset links.
func (m *Manager) HandleLoginError(res http.ResponseWriter, req *http.Request) {
	slogctx.FromCtx(req.Context()).Error("Auth0 reported a login error.",
		slog.String("client_id", req.URL.Query().Get("client_id")),
		slog.String("error_code", req.URL.Query().Get("error")),
		slog.String("error_description", req.URL.Query().Get("error_description")),
		slog.String("tracking", req.URL.Query().Get("tracking")),
	)
	RenderExternalPage(&AccountIssue{}).ServeHTTP(res, req)
}

// HandleRefreshToken handles refreshing the user's access token (using a refresh token) when it is about to expire.
func (m *Manager) HandleRefreshToken(
	httpclient *resty.Client,
	authenticator *auth0.Authenticator,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the refresh token and expiry from the m.SessionMgr.
		tkn, err := authenticator.GetRefreshToken(req.Context(), m.SessionMgr)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get refresh token from m.SessionMgr: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}
		expiry, err := authenticator.GetTokenExpiry(req.Context(), m.SessionMgr)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get token expiry from m.SessionMgr: %w", err),
				StatusCode:    http.StatusBadRequest,
			}).ServeHTTP(res, req)
			return
		}

		// If token will expire soon, refresh it.
		const refreshGracePeriod = time.Hour
		if expiry.UTC().Sub(time.Now().UTC()) < refreshGracePeriod {
			token, err := authenticator.RefreshTokens(req.Context(), httpclient, tkn)
			if err != nil {
				authenticator.ClearAuth(req.Context(), m.SessionMgr)
				http.Redirect(res, req, "/login", http.StatusSeeOther)
				return
			}

			// Save tokens to m.SessionMgr.
			authenticator.SaveTokens(req.Context(), m.SessionMgr, token)

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
