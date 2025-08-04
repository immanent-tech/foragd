// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
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
		slogctx.FromCtx(req.Context()).Debug("Authenticating user.")
		if err := a.auth.CompleteUserAuth(res, req); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Authentication required.", slog.Any("error", err))
			url, err := a.auth.GetAuthURL(req)
			if err != nil {
				RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
				return
			}
			slogctx.FromCtx(req.Context()).Debug("Redirecting to provider.", slog.String("url", url))
			http.Redirect(res, req, url, http.StatusTemporaryRedirect)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("Redirecting to home page.")
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
		if err := a.auth.CompleteUserAuth(res, req); err != nil {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("Authenticated.")
		slogctx.FromCtx(req.Context()).Debug("Redirecting to home page.")
		req.Header.Add("Content-Type", "")

		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}).ServeHTTP
}
