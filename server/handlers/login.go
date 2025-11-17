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

// type authAPI interface {
// 	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
// 	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
// }

// Login handles login requests.
func Login() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		err := auth0.InitAuthenticator(req.Context())
		if err != nil {
			http.Error(res, "Login is not available.", http.StatusServiceUnavailable)
			return
		}

		// prompt=login&screen_hint=signup
		state, err := generateRandomState()
		if err != nil {
			template := templates.Page("Foragd", templates.ErrorPage(models.NewErrorMessage("Unable to log in.", "")))
			err := template.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
			return
		}
		var authURL string
		switch chi.RouteContext(req.Context()).RoutePattern() {
		case "/signup":
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
	}).ServeHTTP
}

// LoginCallback handles processing the response from a login provider.
//
//nolint:gocognit,nestif
func LoginCallback(storeAPI *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// provider := chi.URLParam(req, "provider")

		state := req.FormValue("state")
		if state != session.Manager.GetString(req.Context(), "state") {
			template := templates.Page("Foragd", templates.ErrorPage(models.NewErrorMessage("Unable to log in.", "Invalid state parameter")))
			err := template.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
			return
		}

		// Exchange an authorization code for a token.
		code := req.FormValue("code")
		token, err := auth0.AuthClient.Exchange(req.Context(), code)
		if err != nil {
			template := templates.Page("Foragd", templates.ErrorPage(models.NewErrorMessage("Unable to log in.", "Failed to exchange an authorization code for a token.")))
			err := template.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
			return
		}

		idToken, err := auth0.AuthClient.VerifyIDToken(req.Context(), token)
		if err != nil {
			template := templates.ErrorPage(models.NewErrorMessage("Unable to log in.", "Failed to verify ID Token."))
			renderPage(template, "").ServeHTTP(res, req)
			return
		}

		var profile auth0.UserProfile
		err = idToken.Claims(&profile)
		if err != nil {
			template := templates.Page("Foragd", templates.ErrorPage(models.NewErrorMessage("Unable to log in.", err.Error())))
			err := template.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
			return
		}

		session.Manager.Put(req.Context(), "access_token", token.AccessToken)
		session.Manager.Put(req.Context(), "profile", profile)

		_, err = storeAPI.FindUserByExternalID(req.Context(), profile.GetID())
		if err != nil {
			var apiError models.APIError
			// If a local user is not found, create one.
			if errors.As(err, &apiError) {
				if apiError.StatusCode == http.StatusNotFound {
					user := models.NewUser(profile.GetID(), profile.GetEmail(), "auth0", models.UserLevelStandard)
					valid, err := user.Valid(req.Context())
					if err != nil || !valid {
						template := templates.Page("Foragd", templates.ErrorPage(models.NewErrorMessage("Unable to log in.", err.Error())))
						err := template.Render(req.Context(), res)
						if err != nil {
							slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
							http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
							return
						}
						return
					}
					err = storeAPI.CreateUser(req.Context(), user)
					if err != nil {
						template := templates.Page("Foragd", templates.ErrorPage(models.NewErrorMessage("Unable to log in.", err.Error())))
						err := template.Render(req.Context(), res)
						if err != nil {
							slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
							http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
							return
						}
						return
					}
					slogctx.FromCtx(req.Context()).Debug("Created new local user.")
				}
			} else {
				template := templates.Page("Foragd", templates.ErrorPage(models.NewErrorMessage("Unable to log in.", err.Error())))
				err := template.Render(req.Context(), res)
				if err != nil {
					slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
					http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
					return
				}
				return
			}
		}
		// Sync user data from the backend.
		syncLocalUser(req.Context(), storeAPI, profile)
		// slogctx.FromCtx(req.Context()).Debug("Redirecting")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}).ServeHTTP
}

// syncLocalUser tries to sync relevant user data from the auth backend to the local data.
func syncLocalUser(ctx context.Context, api *elastic.API, profile auth0.UserProfile) {
	user, err := api.FindUserByExternalID(ctx, profile.GetID())
	if err != nil {
		slogctx.FromCtx(ctx).Error("Could not sync user data.",
			slog.Any("error", err))
		return
	}
	ctx = models.UserToCtx(ctx, user)
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
		err = api.UpdateUser(ctx, user.GetID(), updates)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not sync user data.",
				slog.Any("error", err))
			return
		}
	}
}

func generateRandomState() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("unable to generate random state: %w", err)
	}

	state := base64.StdEncoding.EncodeToString(bytes)

	return state, nil
}
