// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/joshuar/go-feed-me/internal/platforms/auth0"
)

// Logout handler handles user logout.
func (s Server) Logout(res http.ResponseWriter, req *http.Request, provider string) {
	logger := s.Logger.With(slog.String("handler", chi.RouteContext(req.Context()).RoutePath))

	switch provider {
	case "auth0":
		auth0.LogoutHandler(res, req)
	default:
		logger.Error("No provider to satisfy login.")
		http.NotFound(res, req)
	}
}
