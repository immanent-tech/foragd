// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/yassinebenaid/godump"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
	"github.com/joshuar/go-feed-me/internal/server/middlewares"
)

var ErrFeedIDRequired = errors.New("feed ID is required")

func (s Server) GetFeedList(res http.ResponseWriter, req *http.Request, params GetFeedListParams) {
	logger := s.Logger.With(slog.String("handler", "ListFeeds"))
	ctx := logging.ToContext(req.Context(), logger)

	godump.Dump(params)

	// Only continue if request was made by htmx.
	if !htmx.IsHTMX(req) {
		logger.Error("Request was not made by htmx.")
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	// Only continue if user is authorized.
	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	if params.FeedsFilters != nil {
		// Add any feeds for filtering.
		if len(params.FeedsFilters.Feeds) > 0 {
			ctx = handlers.FeedsToCtx(ctx, params.FeedsFilters.Feeds)
		}
		// Add categories for filtering.
		if len(params.FeedsFilters.Categories) > 0 {
			ctx = handlers.CategoriesToCtx(ctx, params.FeedsFilters.Categories)
		}
	}

	logging.LogReq(req, http.StatusAccepted).Info("processing request")

	handlers.ShowFeeds(res, req.WithContext(ctx), s.API.elastic, s.API.pg)
}

func (s Server) UpdateFeedList(res http.ResponseWriter, req *http.Request) {
	// logger := s.Logger.With(slog.String("handler", "UpdateFeedList"))
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ListItems(res http.ResponseWriter, req *http.Request, params ListItemsParams) {
	logger := s.Logger.With(slog.String("handler", "ListFeeds"))
	ctx := logging.ToContext(req.Context(), logger)

	// Only continue if request was made by htmx.
	if !htmx.IsHTMX(req) {
		logger.Error("Request was not made by htmx.")
		http.Error(res, "Invalid request", http.StatusBadRequest)
		return
	}

	// Only continue if user is authorized.
	if authenticated, err := middlewares.IsAuthenticated(req, s.API.pg); !authenticated {
		logger.Error("Unauthorized.", slog.Any("error", err))
		res.WriteHeader(http.StatusUnauthorized)
		return
	}

	if params.ItemsFilters != nil {

		// Add any feeds for filtering.
		if len(params.ItemsFilters.Feeds) > 0 {
			ctx = handlers.FeedsToCtx(ctx, params.ItemsFilters.Feeds)
		}
		// Add any items for filtering.
		if len(params.ItemsFilters.Items) > 0 {
			ctx = handlers.FeedsToCtx(ctx, params.ItemsFilters.Items)
		}
		// Add categories for filtering.
		if len(params.ItemsFilters.Categories) > 0 {
			ctx = handlers.CategoriesToCtx(ctx, params.ItemsFilters.Categories)
		}
	}

	handlers.ShowItems(res, req.WithContext(ctx), s.API.elastic, s.API.pg)
}

func (s Server) UpdateItemsList(res http.ResponseWriter, req *http.Request) {
	// logger := s.Logger.With(slog.String("handler", "UpdateFeedList"))
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ShowFeed(res http.ResponseWriter, req *http.Request, feedID string) {
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

func (s Server) ShowItem(res http.ResponseWriter, req *http.Request, feedID string, itemID string) {
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
