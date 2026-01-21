// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/oauth2"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/auth0"
	"github.com/immanent-tech/foragd/server/session"
	"github.com/immanent-tech/foragd/web/templates"
)

// Login handles login requests.
func Login(res http.ResponseWriter, req *http.Request) {
	// Init the authenticator backend.
	if err := auth0.InitAuthenticator(req.Context()); err != nil {
		slogctx.FromCtx(req.Context()).Error("Unable to initialise authenticator backend.",
			slog.Any("error", err),
		)
		renderPage(
			templates.NewPage(
				templates.ErrorMessage(models.NewErrorMessage("Unable to log in.", "Can't contact auth backend")),
			),
		).ServeHTTP(res, req)
	}

	// Retrieve existing stored state or generate new state.
	var state string
	state, err := session.Restore[string](req.Context(), "state")
	if err != nil || state == "" {
		slogctx.FromCtx(req.Context()).Debug("No existing or invalid previous state. Using new state.")
		state, err = auth0.GenerateRandomState()
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Generate new state failed.",
				slog.Any("error", err),
			)
			renderPage(
				templates.NewPage(
					templates.ErrorMessage(models.NewErrorMessage("Unable to log in.", "Invalid state")),
				),
			).ServeHTTP(res, req)
		}
	}

	// Redirect the user appropriately.
	var authURL string
	switch chi.RouteContext(req.Context()).RoutePattern() {
	case "/signup":
		// Retrieve and save the selected plan id into the session for later use.
		planID := req.URL.Query().Get(models.ParamPlanID)
		session.Save(req.Context(), models.ParamPlanID, planID)
		authURL = auth0.AuthClient.AuthCodeURL(state,
			oauth2.SetAuthURLParam("screen_hint", "signup"),
		)
	case "/login":
		authURL = auth0.AuthClient.AuthCodeURL(state)
	}
	session.Save(req.Context(), "state", state)
	slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.",
		slog.String("url", auth0.AuthClient.AuthCodeURL(state)),
	)
	http.Redirect(res, req, authURL, http.StatusTemporaryRedirect)
}

// LoginCallback handles processing the response from a login provider.
func LoginCallback(res http.ResponseWriter, req *http.Request) {
	state, err := session.Restore[string](req.Context(), "state")
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Restore state from session failed.",
			slog.Any("error", err),
		)
		renderPage(
			templates.NewPage(
				templates.ErrorMessage(
					models.NewErrorMessage("Unable to log in.", "This might be a temporary error, please try again."),
				),
			),
		).ServeHTTP(res, req)
	}
	if req.FormValue("state") != state {
		slogctx.FromCtx(req.Context()).Error("Invalid state.")
		renderPage(
			templates.NewPage(
				templates.ErrorMessage(
					models.NewErrorMessage("Unable to log in.", "This might be a temporary error, please try again."),
				),
			),
		).ServeHTTP(res, req)
	}

	// Exchange an authorization code for a token.
	token, err := auth0.AuthClient.Exchange(req.Context(), req.FormValue("code"))
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Unable to exchange auth token.",
			slog.Any("error", err),
		)
		renderPage(
			templates.NewPage(
				templates.ErrorMessage(
					models.NewErrorMessage("Unable to log in.", "This might be a temporary error, please try again."),
				),
			),
		).ServeHTTP(res, req)
	}

	// Verify token.
	idToken, err := auth0.AuthClient.VerifyIDToken(req.Context(), token)
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Unable to verify token.",
			slog.Any("error", err),
		)
		renderPage(
			templates.NewPage(
				templates.ErrorMessage(
					models.NewErrorMessage("Unable to log in.", "This might be a temporary error, please try again."),
				),
			),
		).ServeHTTP(res, req)
	}
	// Save token details to session
	session.Save(req.Context(), "token", *token)

	// Extract user profile.
	var profile auth0.UserProfile
	err = idToken.Claims(&profile)
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Invalid authorization data.",
			slog.Any("error", err),
		)
		renderPage(
			templates.NewPage(
				templates.ErrorMessage(
					models.NewErrorMessage("Unable to log in.", "This might be a temporary error, please try again."),
				),
			),
		).ServeHTTP(res, req)
	}
	// Save profile to session.
	session.Save(req.Context(), "profile", profile)

	var user *models.User
	user, err = models.GetUserByExternalID(req.Context(), profile.GetID())
	switch {
	case err != nil && models.HTTPStatus(err) == http.StatusNotFound: // No local user.
		// Create a new local account for the user
		var err error
		user, err = models.CreateUser(req.Context(), profile.GetID(), profile.GetEmail())
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Unable to create new local user.",
				slog.Any("error", err),
			)
			renderPage(
				templates.NewPage(
					templates.ErrorMessage(models.NewErrorMessage("Unable to log in.", "Account creation failed")),
				),
			).ServeHTTP(res, req)
		}
	case err != nil: // Backend error.
		slogctx.FromCtx(req.Context()).Error("Unable to find a local user match.",
			slog.String("external_user_id", profile.GetID()),
			slog.Any("error", err),
		)
		renderPage(
			templates.NewPage(
				templates.ErrorMessage(
					models.NewErrorMessage("Unable to log in.", "This might be a temporary error, please try again."),
				),
			),
		).ServeHTTP(res, req)
	default: // Existing user.
		// Sync user data from the backend.
		syncLocalUser(req.Context(), user, profile)
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
	if cancelled, endAt := user.Cancelled(); cancelled && endAt.Before(time.Now().UTC()) {
		slogctx.FromCtx(req.Context()).Error("User has cancelled plan.",
			slog.String("user_id", user.GetID()),
			slog.Any("error", err),
		)
		// Account has been cancelled and past cancellation date; redirect to home page.
		http.Redirect(res, req.WithContext(ctx), "/", http.StatusSeeOther)
		return
	}
	// Active user; redirect to home page.
	slogctx.FromCtx(ctx).Info("User logged in.",
		slog.String("user_id", user.GetID()),
	)

	if appState, err := session.Restore[map[string]string](req.Context(), state); err != nil {
		http.Redirect(res, req.WithContext(ctx), models.RouteHome, http.StatusTemporaryRedirect)
	} else {
		if redirectURL, found := appState["redirectURL"]; found {
			slogctx.FromCtx(ctx).Info("Previous state found, redirecting user.",
				slog.String("redirect_url", redirectURL))
			http.Redirect(res, req.WithContext(ctx), redirectURL, http.StatusTemporaryRedirect)
		}
	}

}

// LoginError handles login errors, including invalid login callback URL, missing parameters, expired password reset
// links.
func LoginError(res http.ResponseWriter, req *http.Request) {
	slogctx.FromCtx(req.Context()).Error("Auth0 reported a login error.",
		slog.String("client_id", req.URL.Query().Get("client_id")),
		slog.String("error_code", req.URL.Query().Get("error")),
		slog.String("error_description", req.URL.Query().Get("error_description")),
	)
	renderPage(
		templates.NewPage(
			templates.ErrorMessage(models.NewErrorMessage("Unable to log in.", "Auth backend reported an error")),
		),
	).ServeHTTP(res, req)
}

// syncLocalUser tries to sync relevant user data from the auth backend to the local data.
func syncLocalUser(ctx context.Context, user *models.User, profile auth0.UserProfile) {
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
		if err := models.UpdateUser(ctx, user.GetID(), updates); err != nil {
			slogctx.FromCtx(ctx).Error("Could not sync user data.",
				slog.String("user_id", user.GetID()),
				slog.Any("error", err))
			return
		}
		slogctx.FromCtx(ctx).Info("User data updated.")
	}
}
