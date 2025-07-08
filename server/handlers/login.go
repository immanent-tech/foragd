// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
)

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
				RenderTemplate(RespBackendError(err)).ServeHTTP(res, req)
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
			RenderTemplate(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		slogctx.FromCtx(req.Context()).Debug("Authenticated.")
		slogctx.FromCtx(req.Context()).Debug("Redirecting to home page.")
		req.Header.Add("Content-Type", "")

		http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
	}).ServeHTTP
}
