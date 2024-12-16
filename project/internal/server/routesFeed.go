// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/cookies"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
	"github.com/joshuar/go-feed-me/internal/server/middlewares"
)

var ErrFeedIDRequired = errors.New("feed ID is required")

func (s Server) GetFeedList(res http.ResponseWriter, req *http.Request, params GetFeedListParams) {
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
	logger := s.Logger.With(slog.String("handler", "UpdateItemsList"))

	filters, problems, err := handlers.DecodeForm[*models.ItemsFilters](req)
	if err != nil && len(problems) == 0 {
		logger.Error("Could not decode filters.", slog.Any("error", err))
		return
	}

	cookie, err := setItemsListCookie(filters)
	if err != nil && len(problems) == 0 {
		logger.Error("Could not encode filters.", slog.Any("error", err))
		return
	}

	if err := cookies.WriteEncrypted(res, cookie, []byte(s.AppSecret())); err != nil {
		logger.Error("Could not write cookie.", slog.Any("error", err))
		return
	}

	s.ListItems(res, req, ListItemsParams{ItemsFilters: filters})
	// handlers.ShowItems(res, req, s.API.elastic, s.API.pg)
	// logger := s.Logger.With(slog.String("handler", "UpdateFeedList"))
	// res.WriteHeader(http.StatusNotImplemented)
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

	if feedID == "" || itemID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrFeedIDRequired))
		http.Error(res, "Invalid request.", http.StatusBadRequest)
		return
	}

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

	handlers.ShowItem(res, req, feedID, itemID, s.API.elastic, s.API.pg)
}

func setItemsListCookie(filters *models.ItemsFilters) (http.Cookie, error) {
	// Initialize a buffer to hold the gob-encoded data.
	var buf bytes.Buffer

	// Gob-encode the user data, storing the encoded output in the buffer.
	err := gob.NewEncoder(&buf).Encode(filters)
	if err != nil {
		log.Println(err)
		return http.Cookie{}, fmt.Errorf("could not encode cookie value: %w", err)
	}

	return http.Cookie{
		Name:     "itemsFilters",
		Value:    buf.String(),
		Path:     "/home/items",
		MaxAge:   3600,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}, nil
}
