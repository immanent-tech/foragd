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
	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/cookies"
	"github.com/joshuar/go-feed-me/internal/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates/content"
)

var ErrFeedIDRequired = errors.New("feed ID is required")

func (s Server) GetFeedList(res http.ResponseWriter, req *http.Request, params GetFeedListParams) {
	logger := s.Logger.With(slog.String("handler", "ListFeeds"))

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), s.API.elastic, s.API.pg)
	if err != nil {
		logger.Error("Could not add item.", slog.Any("error", err))
	}

	feedCards := make([]components.Card, len(feeds))

	// Generate cards for each feed.
	for i, feed := range feeds {
		feedCards[i] = feed.AsCardSummary()
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, content.ShowFeeds(feedCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showAllFeeds: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func (s Server) UpdateFeedList(res http.ResponseWriter, req *http.Request) {
	// logger := s.Logger.With(slog.String("handler", "UpdateFeedList"))
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ListItems(res http.ResponseWriter, req *http.Request, params ListItemsParams) {
	logger := s.Logger.With(slog.String("handler", "ListItems"))

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), s.API.elastic, s.API.pg, params.ItemsFilters.Feeds...)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not add item.", slog.Any("error", err))
	}

	subs := make([]string, len(feeds))

	// Get a list of the feed IDs.
	for i, feed := range feeds {
		subs[i] = feed.ID
	}
	// Get all feed items for all subscribed feeds.
	items, err := s.API.elastic.GetFeedItems(req.Context(), subs...)
	if err != nil {
		logger.Error("Could not show feed items.", slog.Any("error", err))
	}

	itemCards := make([]components.Card, len(items))

	// Create item cards.
	for i, item := range items {
		itemCards[i] = item.AsCardSummary()
	}
	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, content.ShowFeedItems(itemCards...)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
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

	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ShowItem(res http.ResponseWriter, req *http.Request, feedID string, itemID string) {
	logger := s.Logger.With(slog.String("handler", "FeedItem"))

	if feedID == "" || itemID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrFeedIDRequired))
		http.Error(res, "Invalid request.", http.StatusBadRequest)
		return
	}

	item, err := models.GetItem(req.Context(), s.API.pg, s.API.elastic, feedID, itemID)
	if err != nil {
		logging.FromContext(req.Context()).
			Error("Could not get item.", slog.Any("error", err))
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res, content.ShowItem(item)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
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
