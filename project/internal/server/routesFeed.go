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

func (s Server) GetHomeFeed(res http.ResponseWriter, req *http.Request, feedID string) {
	logger := s.Logger.With(slog.String("handler", "Feed"))

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
	logging.LogReq(req, http.StatusAccepted).Info("processing request")

	handlers.GetFeedHandler(res, req.WithContext(ctx), feedID, s.API.pg, s.API.elastic)
}

func (s Server) GetHomeFeedItem(res http.ResponseWriter, req *http.Request, feedID string, itemID string) {
	logger := s.Logger.With(slog.String("handler", "FeedItem"))

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

	handlers.GetFeedItemHandler(res, req.WithContext(ctx), feedID, itemID, s.API.pg, s.API.elastic)
}
