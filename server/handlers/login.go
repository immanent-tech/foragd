// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/oauth2"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web/templates"
)

// Login handles login requests.
func Login() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		err := auth0.InitAuthenticator(req.Context())
		if err != nil {
			renderPage(
				templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Can't contact auth backend")),
				"Login",
			).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}

		// prompt=login&screen_hint=signup
		state, err := generateRandomState()
		if err != nil {
			renderPage(
				templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Invalid state")),
				"Login",
			).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}
		var authURL string
		switch chi.RouteContext(req.Context()).RoutePattern() {
		case "/signup":
			// Retrieve and save the selected plan id into the session for later use.
			planID := req.URL.Query().Get(models.ParamPlanID)
			session.SaveToSession(req.Context(), models.ParamPlanID, planID)
			authURL = auth0.AuthClient.AuthCodeURL(state,
				oauth2.SetAuthURLParam("screen_hint", "signup"),
			)
		case "/login":
			authURL = auth0.AuthClient.AuthCodeURL(state)
		}
		session.Manager.Put(req.Context(), "state", state)
		slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.",
			slog.String("url", auth0.AuthClient.AuthCodeURL(state)),
		)
		http.Redirect(res, req, authURL, http.StatusTemporaryRedirect)
		return nil
	})).ServeHTTP
}

// LoginCallback handles processing the response from a login provider.
func LoginCallback(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// provider := chi.URLParam(req, "provider")

		state := req.FormValue("state")
		if state != session.Manager.GetString(req.Context(), "state") {
			renderPage(
				templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Invalid state parameter")),
				"Login",
			).ServeHTTP(res, req)
			return models.NewAPIError(errors.New("invalid session state"), http.StatusInternalServerError)
		}

		// Exchange an authorization code for a token.
		code := req.FormValue("code")
		token, err := auth0.AuthClient.Exchange(req.Context(), code)
		if err != nil {
			renderPage(
				templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Invalid authorization data")),
				"Login",
			).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}

		idToken, err := auth0.AuthClient.VerifyIDToken(req.Context(), token)
		if err != nil {
			renderPage(
				templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Invalid authorization data")),
				"Login",
			).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}

		var profile auth0.UserProfile
		err = idToken.Claims(&profile)
		if err != nil {
			renderPage(
				templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Invalid authorization data")),
				"Login",
			).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		}

		session.Manager.Put(req.Context(), "access_token", token.AccessToken)
		session.Manager.Put(req.Context(), "profile", profile)

		user, err := api.FindUserByExternalID(req.Context(), profile.GetID())
		switch {
		case err != nil && models.HTTPStatus(err) == http.StatusNotFound: // No local user.
			// Create a new local account for the user
			if err := createLocalUser(req.Context(), api, profile); err != nil {
				renderPage(
					templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Account creation failed")),
					"Login",
				).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusInternalServerError)
			}
		case err != nil: // Backend error.
			renderPage(
				templates.ExternalError(models.NewErrorMessage("Unable to log in.", "Authorization backend error")),
				"Login",
			).ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusInternalServerError)
		default: // Existing user.
			// Sync user data from the backend.
			syncLocalUser(req.Context(), api, user, profile)
		}
		ctx := models.UserToCtx(req.Context(), user)
		// Redirect the user appropriately.
		if profile.LoginsCount == 1 || user.Metadata.Plan == "" {
			// New user or user without a plan; redirect to choose subscription plan.
			http.Redirect(res, req.WithContext(ctx), models.RouteCheckoutChoosePlan, http.StatusSeeOther)
			return nil
		}
		if err := user.Metadata.Valid(); err != nil {
			// User metadata is invalid, redirect user to page indicating they need to contact support to resolve the issue.
			http.Redirect(res, req.WithContext(ctx), models.RouteUserAccountIssue, http.StatusSeeOther)
			return fmt.Errorf("checking user data: %w", err)
		}
		if !user.Active() {
			// Account issues; redirect user to page indicating they need to contact support to resolve an issue with their account.
			http.Redirect(res, req.WithContext(ctx), models.RouteUserAccountIssue, http.StatusSeeOther)
			return nil
		}
		if cancelled, endAt := user.Cancelled(); cancelled && endAt.Before(time.Now().UTC()) {
			// Account has been cancelled and past cancellation date; redirect to home page.
			http.Redirect(res, req.WithContext(ctx), "/", http.StatusSeeOther)
			return nil
		}
		// Active user; redirect to home page.
		http.Redirect(res, req.WithContext(ctx), models.RouteHome, http.StatusTemporaryRedirect)
		return nil
	})).ServeHTTP
}

func createLocalUser(ctx context.Context, api *elastic.API, profile auth0.UserProfile) error {
	user := models.NewUser(profile.GetID(), profile.GetEmail())
	valid, err := user.Valid(ctx)
	if err != nil || !valid {
		return fmt.Errorf("create local user: %w", err)
	}
	err = api.CreateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("create local user: %w", err)
	}

	return nil
}

// syncLocalUser tries to sync relevant user data from the auth backend to the local data.
func syncLocalUser(ctx context.Context, api *elastic.API, user *models.User, profile auth0.UserProfile) {
	ctx = elastic.SetupIndexAliases(ctx)
	// Create needed updates by comparing request values to existing user values and adding new values to updates map as appropriate.
	updates := make(map[string]any)
	// Overwrite local avatar with remote avatar if different
	if user.AvatarURL != profile.Picture {
		updates["avatar_url"] = profile.Picture
	}
	// Overwrite local nickname with remote nickname if different
	if user.Nickname != profile.Nickname {
		updates["nickname"] = profile.Nickname
	}
	// Overwrite local email with remote email if different
	if user.Email != profile.Email {
		updates["email"] = profile.Email
	}
	// If no updates are necessary, bail early.
	if len(updates) > 0 {
		err := api.UpdateUser(ctx, user.GetID(), updates)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not sync user data.",
				slog.Any("error", err))
			return
		}
	}
}

func generateRandomState() (string, error) {
	const stateSize = 32
	bytes := make([]byte, stateSize)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("unable to generate random state: %w", err)
	}

	state := base64.StdEncoding.EncodeToString(bytes)

	return state, nil
}
