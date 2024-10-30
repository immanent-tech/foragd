// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
	"github.com/joshuar/go-feed-me/internal/server/middlewares"
)

// UserHome serves the user home page.
// GET(/home)
func (s Server) UserHome(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "Home"))

	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		http.Redirect(res, req, "/", http.StatusSeeOther)
		return
	}

	ctx := logging.ToContext(req.Context(), logger)

	handlers.Home(res, req.WithContext(ctx))
}

func (s Server) UserSearch(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "UserSearch"))

	if !htmx.IsHTMX(req) {
		logger.Error("Request was not made by htmx.")
		http.Error(res, "Invalid request", http.StatusBadRequest)
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

func (s Server) UserSettings(res http.ResponseWriter, req *http.Request) {
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

func (s Server) UserHomeFeed(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "UserHomeFeed"))

	// if !htmx.IsHTMX(req) {
	// 	logger.Error("Request was not made by htmx.")
	// 	http.Error(res, "Invalid request", http.StatusBadRequest)
	// 	return
	// }

	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	ctx := logging.ToContext(req.Context(), logger)

	handlers.HomeFeed(res, req.WithContext(ctx), s.API.websocket)
}
