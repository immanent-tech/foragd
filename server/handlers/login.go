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

	"github.com/coreos/go-oidc/v3/oidc"
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

type authAPI interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error)
}

// Login handles login requests.
func Login(authAPI authAPI) http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		state, err := generateRandomState()
		if err != nil {
			template := templates.Page("Foragd", templates.Error(models.NewErrorMessage("Unable to log in.", "")))
			err := template.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
			return
		}
		session.Manager.Put(req.Context(), "state", state)
		slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.",
			slog.String("provider", provider),
			slog.String("url", authAPI.AuthCodeURL(state)),
		)
		http.Redirect(res, req, authAPI.AuthCodeURL(state), http.StatusTemporaryRedirect)
	}).ServeHTTP
}

// LoginCallback handles processing the response from a login provider.
func LoginCallback(authAPI authAPI, storeAPI *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// provider := chi.URLParam(req, "provider")

		state := req.FormValue("state")
		if state != session.Manager.GetString(req.Context(), "state") {
			template := templates.Page("Foragd", templates.Error(models.NewErrorMessage("Unable to log in.", "Invalid state parameter")))
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
		token, err := authAPI.Exchange(req.Context(), code)
		if err != nil {
			template := templates.Page("Foragd", templates.Error(models.NewErrorMessage("Unable to log in.", "Failed to exchange an authorization code for a token.")))
			err := template.Render(req.Context(), res)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
				http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
				return
			}
			return
		}

		idToken, err := authAPI.VerifyIDToken(req.Context(), token)
		if err != nil {
			template := templates.Error(models.NewErrorMessage("Unable to log in.", "Failed to verify ID Token."))
			renderPage(template, "").ServeHTTP(res, req)
			return
		}

		var profile auth0.UserProfile
		err = idToken.Claims(&profile)
		if err != nil {
			template := templates.Page("Foragd", templates.Error(models.NewErrorMessage("Unable to log in.", err.Error())))
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
					err = createLocalUser(req.Context(), storeAPI, profile.GetID())
					if err != nil {
						template := templates.Page("Foragd", templates.Error(models.NewErrorMessage("Unable to log in.", err.Error())))
						err := template.Render(req.Context(), res)
						if err != nil {
							slogctx.FromCtx(req.Context()).Error("Failed to render page template.", slog.Any("error", err))
							http.Error(res, "Failed to render page template.", http.StatusInternalServerError)
							return
						}
						return
					}
					slogctx.FromCtx(req.Context()).Debug("Create new local user.")
				}
			} else {
				template := templates.Page("Foragd", templates.Error(models.NewErrorMessage("Unable to log in.", err.Error())))
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
	// For the following fields, assume that if the backend value is different from the local value, it was updated on
	// the backend. In such cases, replace the local value.
	updates := make(map[string]any)
	// Overwrite local avatar with remote avatar if different
	if user.AvatarURL != profile.Picture {
		updates["avatar_url"] = profile.Picture
	}
	// Overwrite local nickname with remote nickname if different
	if user.Nickname != profile.Nickname {
		updates["nickname"] = profile.Nickname
	}
	if len(updates) > 0 {
		// Update the user object.
		err := api.UpdateUser(ctx, user.GetID(), updates)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Could not sync user data.",
				slog.Any("error", err))
		} else {
			slogctx.FromCtx(ctx).Debug("User data synced.")
		}
	}
}

// createLocaUser will create a local user for user that has authenticated via the auth backend.
func createLocalUser(ctx context.Context, api *elastic.API, externalID string) error {
	user := models.NewUser(externalID, "auth0")
	valid, err := user.Valid(ctx)
	if err != nil || !valid {
		return fmt.Errorf("cannot create local user: %w", err)
	}
	index, err := elastic.UserWriteIndexFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("cannot create local user: %w", err)
	}
	err = elastic.CreateDoc(ctx, api.GetAPI(), index, user.GetID(), user)
	if err != nil {
		return fmt.Errorf("cannot create local user: %w", err)
	}
	return nil
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("unable to generate random state: %w", err)
	}

	state := base64.StdEncoding.EncodeToString(b)

	return state, nil
}
