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
)

// Login handles login requests.
func Login(api models.AuthAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		api.SetProviderName(req.Context(), provider)
		alice.New(
			RouteLogger,
		).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
			slogctx.FromCtx(req.Context()).Debug("Authenticating user.")
			if err := api.CompleteUserAuth(res, req); err != nil {
				slogctx.FromCtx(req.Context()).Warn("Authentication required.", slog.Any("error", err))
				url, err := api.GetAuthURL(req)
				if err != nil {
					ProcessResponse(res, req, models.NewResponse(http.StatusInternalServerError, err))
					return
				}
				slogctx.FromCtx(req.Context()).Debug("Redirecting to provider.", slog.String("url", url))
				http.Redirect(res, req, url, http.StatusTemporaryRedirect)
				return
			}
			slogctx.FromCtx(req.Context()).Debug("Redirecting to home page.")
			req.Header.Add("Content-Type", "")
			http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
		}).ServeHTTP(res, req)
	}
}

// LoginCallback handles processing the response from a login provider.
func LoginCallback(api models.AuthAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		provider := chi.URLParam(req, "provider")
		api.SetProviderName(req.Context(), provider)
		alice.New(
			RouteLogger,
		).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := api.CompleteUserAuth(res, req); err != nil {
				ProcessResponse(res, req, models.NewResponse(http.StatusInternalServerError, err))
				return
			}
			slogctx.FromCtx(req.Context()).Debug("Authenticated.")
			slogctx.FromCtx(req.Context()).Debug("Redirecting to home page.")
			req.Header.Add("Content-Type", "")

			http.Redirect(res, req, "/home", http.StatusTemporaryRedirect)
		}).ServeHTTP(res, req)
	}
}
