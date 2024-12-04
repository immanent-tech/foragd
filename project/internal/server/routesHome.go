// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"log/slog"
	"net/http"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
	"github.com/joshuar/go-feed-me/internal/server/middlewares"
)

// GetHome serves the user home page.
// GET(/home).
func (s Server) GetHome(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "Home"))

	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		http.Redirect(res, req, "/", http.StatusSeeOther)
		return
	}

	ctx := logging.ToContext(req.Context(), logger)

	handlers.Home(res, req.WithContext(ctx), s.API.elastic, s.API.pg)
}

func (s Server) GetHomeSettings(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "UserSettings"))

	if err := middlewares.RequireHtmx(res, req); err != nil {
		logger.Error("Request failed.", slog.Any("error", err))
		return
	}

	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	ctx := logging.ToContext(req.Context(), logger)

	handlers.Search(res, req.WithContext(ctx))
}
