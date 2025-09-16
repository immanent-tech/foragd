// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-feed-me/models"
	"github.com/immanent-tech/go-feed-me/providers/elastic"
	"github.com/immanent-tech/go-feed-me/providers/elastic/schema"
	"github.com/immanent-tech/go-feed-me/web/templates/layouts"
	"github.com/immanent-tech/go-feed-me/web/templates/partials"
)

// LoginSelect handles showing options for logging in with different providers.
func LoginSelect() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		page := &layouts.Login{}
		renderPage(page.Content(), "Login - Go Feed Me").ServeHTTP(res, req)
	}).ServeHTTP
}

// Login handles login requests.
func (a *API) Login() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		a.Auth.SetProviderName(req.Context(), provider)
		err := a.Auth.CompleteUserAuth(res, req)
		if err != nil {
			url, err := a.Auth.GetAuthURL(req)
			if err != nil {
				template := partials.Error(models.NewErrorMessage("Unable to log in.", ""))
				renderPage(template, "").ServeHTTP(res, req)
				return
			}
			slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.",
				slog.String("provider", provider),
				slog.String("url", url),
			)
			http.Redirect(res, req, url, http.StatusTemporaryRedirect)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("User logged in.")
		// Sync user data from the backend.
		a.syncLocalUser(req.Context())
		req.Header.Add("Content-Type", "")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}).ServeHTTP
}

// LoginCallback handles processing the response from a login provider.
func (a *API) LoginCallback() http.HandlerFunc {
	return alice.New(
		routeLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		a.Auth.SetProviderName(req.Context(), provider)
		err := a.Auth.CompleteUserAuth(res, req)
		if err != nil {
			template := partials.Error(models.NewErrorMessage("Unable to log in.", ""))
			renderPage(template, "").ServeHTTP(res, req)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("User logged in to auth backend.")
		userAuth, found := a.Auth.GetUserAuth(req.Context())
		if !found {
			template := partials.Error(models.NewErrorMessage("Unable to log in.", ""))
			renderPage(template, "").ServeHTTP(res, req)
			return
		}
		_, err = a.DataAPI().FindUserByExternalID(req.Context(), userAuth.GetUserID())
		if err != nil {
			var apiError models.APIError
			// If a local user is not found, create one.
			if errors.As(err, &apiError) {
				if apiError.StatusCode == http.StatusNotFound {
					err = createLocalUser(req.Context(), a.DataAPI(), userAuth.GetUserID())
					if err != nil {
						template := partials.Error(models.NewErrorMessage("Unable to log in.", ""))
						renderPage(template, "").ServeHTTP(res, req)
						return
					}
					slogctx.FromCtx(req.Context()).Debug("Create new local user.")
				}
			} else {
				template := partials.Error(models.NewErrorMessage("Unable to log in.", ""))
				renderPage(template, "").ServeHTTP(res, req)
				return
			}
		}
		req.Header.Add("Content-Type", "")
		// Sync user data from the backend.
		a.syncLocalUser(req.Context())
		slogctx.FromCtx(req.Context()).Debug("Redirecting")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}).ServeHTTP
}

// syncLocalUser tries to sync relevant user data from the auth backend to the local data.
func (a *API) syncLocalUser(ctx context.Context) {
	// Get the backend data.
	userAuth, found := a.Auth.GetUserAuth(ctx)
	if !found {
		slogctx.FromCtx(ctx).Error("Could not sync user data, user auth not found.")
		return
	}
	user, err := a.DataAPI().FindUserByExternalID(ctx, userAuth.GetUserID())
	if err != nil {
		slogctx.FromCtx(ctx).Error("Could not sync user data.",
			slog.Any("error", err))
		return
	}
	ctx = models.UserToCtx(ctx, user)
	ctx = elastic.UserIndexToCtx(ctx, schema.UsersSchemaPrefix)
	// For the following fields, assume that if the backend value is different from the local value, it was updated on
	// the backend. In such cases, replace the local value.
	updates := make(map[string]any)
	// Overwrite local avatar with remote avatar if different
	if user.AvatarURL != userAuth.AvatarURL {
		updates["avatar_url"] = userAuth.AvatarURL
	}
	// Overwrite local nickname with remote nickname if different
	if user.Nickname != userAuth.NickName {
		updates["nickname"] = userAuth.NickName
	}
	if len(updates) > 0 {
		// Update the user object.
		err := a.updateUser(ctx, updates)
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
	index := elastic.UserIndexFromCtx(ctx)
	if index == "" {
		return fmt.Errorf("cannot create local user: %w", ErrNoCtxData)
	}
	err = elastic.CreateDoc(ctx, api.GetAPI(), index, user.GetID(), user)
	if err != nil {
		return fmt.Errorf("cannot create local user: %w", err)
	}
	return nil
}
