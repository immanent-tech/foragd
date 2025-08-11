// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
	"github.com/joshuar/go-feed-me/web/templates/pages"
)

// LoginSelect handles showing options for logging in with different providers.
func LoginSelect() http.HandlerFunc {
	page := &pages.Login{}
	return alice.New(
		RouteLogger,
	).Then(RenderResponse(models.NewResponse(
		models.WithResponseTemplate(page.Template()),
	))).ServeHTTP
}

// Login handles login requests.
func (a *API) Login() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		a.auth.SetProviderName(req.Context(), provider)
		err := a.auth.CompleteUserAuth(res, req)
		if err != nil {
			url, err := a.auth.GetAuthURL(req)
			if err != nil {
				RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
				return
			}
			slogctx.FromCtx(req.Context()).Debug("Authentication required, redirecting to provider.",
				slog.String("provider", provider))
			http.Redirect(res, req, url, http.StatusTemporaryRedirect)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("User logged in.")
		// Sync user data from the backend.
		a.syncUser(req.Context())
		req.Header.Add("Content-Type", "")
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}).ServeHTTP
}

// LoginCallback handles processing the response from a login provider.
func (a *API) LoginCallback() http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		a.auth.SetProviderName(req.Context(), provider)
		err := a.auth.CompleteUserAuth(res, req)
		if err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("User logged in.")
		req.Header.Add("Content-Type", "")
		// Sync user data from the backend.
		a.syncUser(req.Context())
		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}).ServeHTTP
}

// syncUser tries to sync relevant user data from the auth backend to the local data.
func (a *API) syncUser(ctx context.Context) {
	// Get the backend data.
	userAuth, found := a.auth.GetUserAuth(ctx)
	if !found {
		slogctx.FromCtx(ctx).Error("Could not sync user data, user auth not found.")
		return
	}
	// Get the user.
	users, _, err := elastic.Search[*models.User](ctx, a.DataAPI().GetAPI(), schema.UsersSchemaPrefix, query.Term("external_user_id", userAuth.GetUserID()), 1)
	if err != nil || len(users) == 0 {
		slogctx.FromCtx(ctx).Error("Could not sync user data, user not found.")
		return
	}
	user := users[0]
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
			slogctx.FromCtx(ctx).Error("Failed to sync user data.",
				slog.Any("error", err))
		} else {
			slogctx.FromCtx(ctx).Debug("User data synced.")
		}
	}
}
