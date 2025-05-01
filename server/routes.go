// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/settings"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// Index handler handles the index page.
func (s Server) Index(res http.ResponseWriter, req *http.Request) {
	layout := &layouts.IndexLayout{}
	handlers.HTMXResponse(htmx.NewResponse(), layout.FullRender()).ServeHTTP(res, req)
}

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
	var backLink *models.Route
	prevPage, ok := s.SessionAPI().Get(req.Context(), handlers.HomeHistorySessionKey).(string)
	if !ok {
		backLink = models.NewRoute("/home", nil)
	} else {
		backLink = models.NewRoute(prevPage, nil)
	}
	layout := settings.BuildSettingsLayout(partials.GenerateBackButton(backLink))
	handler := handlers.HTMXResponse(htmx.NewResponse(), layout.FullRender())
	handler.ServeHTTP(res, req)
}

// Logout handler handles user logout.
func (s Server) Logout(res http.ResponseWriter, req *http.Request) {
	s.AuthAPI().Logout().ServeHTTP(res, req)
}
