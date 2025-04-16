// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"net/http"

	"github.com/joshuar/go-feed-me/cmd/server/handlers"
	"github.com/joshuar/go-feed-me/internal/auth"
)

// Login handler handles login requests.
func (s Server) Login(res http.ResponseWriter, req *http.Request, provider string) {
	ctx := AddRouteLogger(req.Context(), "login")
	ctx = auth.ProviderToCtx(ctx, provider)
	handlers.Login(s.AuthAPI()).ServeHTTP(res, req.WithContext(ctx))
}

// LoginCallback handles the callback from login providers.
func (s Server) LoginCallback(res http.ResponseWriter, req *http.Request, provider string) {
	ctx := AddRouteLogger(req.Context(), "login_callback")
	ctx = auth.ProviderToCtx(ctx, provider)
	handlers.AuthCallback(s.AuthAPI()).ServeHTTP(res, req.WithContext(ctx))
}

func (s Server) GetHomeSettings(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}
