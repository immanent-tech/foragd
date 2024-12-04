// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/yassinebenaid/godump"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
	"github.com/joshuar/go-feed-me/internal/server/middlewares"
)

var ErrFeedIDRequired = errors.New("feed ID is required")

func (s Server) ListFeeds(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "ListFeeds"))
	ctx := logging.ToContext(req.Context(), logger)

	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	filters, problems, err := handlers.DecodeForm[*models.Filters](req)
	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode filters.", slog.Any("error", err))
	}

	if filters == nil {
		filters = &models.Filters{}
	}

	logging.LogReq(req, http.StatusAccepted).Info("processing request")

	handlers.ShowFeeds(res, req.WithContext(ctx), s.API.elastic, s.API.pg, filters)
}

func (s Server) ListItems(res http.ResponseWriter, req *http.Request) {
	logger := s.Logger.With(slog.String("handler", "ListFeeds"))
	// ctx := logging.ToContext(req.Context(), logger)

	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	filters, problems, err := handlers.DecodeForm[*models.Filters](req)
	if err != nil && len(problems) == 0 {
		logging.FromContext(req.Context()).
			Error("Could not decode filters.", slog.Any("error", err))
	}

	godump.Dump(filters)

	// logging.LogReq(req, http.StatusAccepted).Info("processing request")
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) GetFeed(res http.ResponseWriter, req *http.Request, feedID string) {
	logger := s.Logger.With(slog.String("handler", "Feed"))

	if feedID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrFeedIDRequired))
		http.Error(res, "Invalid request.", http.StatusBadRequest)
		return
	}

	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) GetItem(res http.ResponseWriter, req *http.Request, feedID string, itemID string) {
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

	res.WriteHeader(http.StatusNotImplemented)
}
