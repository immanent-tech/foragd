// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/views"
)

func GetSettings() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set layout.
		page := views.SettingsPage{}
		ctx := templateToCtx(req.Context(), page.Show())

		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Display content based on request.
		switch {
		case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
			// Partial update. Only render fragments.
			chain.Then(RenderTemplateFragments("content")).ServeHTTP(res, req.WithContext(ctx))
		default:
			// Full page render.
			chain.Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
		}
	}
}

func SetTheme(api models.DocumentsAPI) http.HandlerFunc {
	return alice.New(
		RouteLogger,
	).ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		theme := chi.URLParam(req, "theme")
		user, found := models.UserFromCtx(req.Context())
		if !found {
			ProcessResponse(res, req, models.RespErrUnauthorized())
			return
		}
		settings := user.GetSettings()
		settings.Theme = theme
		if err := api.UpdateUser(req.Context(), map[string]any{
			"settings":   settings,
			"updated_at": time.Now().UTC(),
		}); err != nil {
			RenderError(res, req, models.RespErrBackend(fmt.Errorf("failed to update theme: %w", err)))
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}
