// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/settings"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// Index handler handles the index page.
func (s Server) Index(res http.ResponseWriter, req *http.Request) {
	layout := &layouts.IndexLayout{}
	handlers.PartialRender(layout.FullRender()).ServeHTTP(res, req)
}

// Login handler handles login requests.
func (s Server) Login(res http.ResponseWriter, req *http.Request, provider string) {
	s.AuthAPI().SetProviderName(req.Context(), provider)
	chain := alice.New(
		handlers.RouteLogger,
	).Then(handlers.PerformAuth(s.AuthAPI()))
	chain.ServeHTTP(res, req)
}

// LoginCallback handles the callback from login providers.
func (s Server) LoginCallback(res http.ResponseWriter, req *http.Request, provider string) {
	s.AuthAPI().SetProviderName(req.Context(), provider)
	chain := alice.New(
		handlers.RouteLogger,
	).Then(handlers.AuthCallback(s.AuthAPI(), s.SessionAPI()))
	chain.ServeHTTP(res, req)
}

// GetSettings handles opening the settings modal.
func (s Server) GetSettings(res http.ResponseWriter, req *http.Request) {
	var handler http.Handler

	switch htmx.IsHTMX(req) {
	case true:
		lastViewed := models.GetLastPageView(req.Context(), s.SessionAPI())
		handler = handlers.BaseChain.Then(
			handlers.PartialRender(
				settings.SettingsHeader(),
				settings.SettingsContent(),
				partials.UpdateBacklink(lastViewed),
				settings.ResetFooter(),
			),
		)
	case false:
		handler = handlers.BaseChain.Then(
			handlers.FullRender("Settings",
				templates.WithBody(settings.NewSettingsLayout()),
			),
		)
	}

	handler.ServeHTTP(res, req)
}

func (s Server) GetTheme(res http.ResponseWriter, req *http.Request) {
	handler := handlers.BaseChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		theme, ok := s.SessionAPI().Get(req.Context(), models.ThemeSessionKey).(string)
		if !ok {
			slogctx.FromCtx(req.Context()).Debug("No theme in session. Using a default.")
			theme = "light"
		}
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(theme))
	})
	handler.ServeHTTP(res, req)
}

func (s Server) SetTheme(res http.ResponseWriter, req *http.Request) {
	theme := req.FormValue("theme")
	handler := handlers.BaseChain.Append(
		handlers.SaveTheme(s.SessionAPI(), theme),
		// handlers.UpdateTheme(s.SessionAPI()),
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		resp := handlers.HTMXResponseFromCtx(req.Context())
		resp.Write(res)
		res.WriteHeader(http.StatusOK)
		res.Write(nil)
	})
	handler.ServeHTTP(res, req)
}

// Logout handler handles user logout.
func (s Server) Logout(res http.ResponseWriter, req *http.Request) {
	s.AuthAPI().Logout().ServeHTTP(res, req)
}
