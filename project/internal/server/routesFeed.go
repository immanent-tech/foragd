// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	components "github.com/joshuar/go-templ-daisyui"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/content"
)

var ErrFeedIDRequired = errors.New("feed ID is required")

func (s Server) FeedsHandler(res http.ResponseWriter, req *http.Request, params FeedsHandlerParams) {
	logger := s.Logger.With(slog.String("handler", "ListFeeds"))

	var feedIDs []string
	// categories []string

	if params.Feeds != nil {
		feedIDs = append(feedIDs, *params.Feeds...)
	}
	// if params.Categories != nil {
	// 	categories = append(feedIDs, *params.Categories...)
	// }

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), s.API.elastic, s.API.pg, feedIDs...)
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

func (s Server) FeedsCategoryHandler(res http.ResponseWriter, req *http.Request, category string) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ItemsHandler(res http.ResponseWriter, req *http.Request, params ItemsHandlerParams) {
	logger := s.Logger.With(slog.String("handler", "ListItems"))

	var feedIDs []string
	// categories []string

	if params.Feeds != nil {
		feedIDs = append(feedIDs, *params.Feeds...)
	}
	// if params.Categories != nil {
	// 	categories = append(feedIDs, *params.Categories...)
	// }

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), s.API.elastic, s.API.pg, feedIDs...)
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

func (s Server) ItemsCategoryHandler(res http.ResponseWriter, req *http.Request, category string) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ArticleHandler(res http.ResponseWriter, req *http.Request, feedID string, itemID string) {
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
