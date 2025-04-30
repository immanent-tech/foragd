// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates/layouts/settings"
)

// Login handler handles login requests.
func (s Server) Login(res http.ResponseWriter, req *http.Request, provider string) {
	s.AuthAPI().SetProviderName(req.Context(), provider)
	chain := alice.New(
		handlers.RouteLogger("show_feeds"),
	).Then(handlers.PerformAuth(s.AuthAPI()))
	chain.ServeHTTP(res, req)
}

// LoginCallback handles the callback from login providers.
func (s Server) LoginCallback(res http.ResponseWriter, req *http.Request, provider string) {
	s.AuthAPI().SetProviderName(req.Context(), provider)
	chain := alice.New(
		handlers.RouteLogger("show_feeds"),
	).Then(handlers.AuthCallback(s.AuthAPI(), s.SessionAPI()))
	chain.ServeHTTP(res, req)
}

// GetSettings handles opening the settings modal.
func (s Server) GetSettings(res http.ResponseWriter, req *http.Request) {
	spew.Dump(req.Referer())
	spew.Dump(s.SessionAPI().Get(req.Context(), handlers.HomeHistorySessionKey))
	handler := handlers.HTMXResponse(htmx.NewResponse(), settings.Page())
	handler.ServeHTTP(res, req)
}

// Logout handler handles user logout.
func (s Server) Logout(res http.ResponseWriter, req *http.Request) {
	s.AuthAPI().Logout().ServeHTTP(res, req)
}
