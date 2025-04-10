// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/cmd/server/handlers"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
)

var (
	ErrGeneratePageNavigationFailed = errors.New("error occurred while generating page navigation")
	ErrParseMarkRequest             = errors.New("could not parse mark request")
)

func (s Server) HandleHome(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleHomeNotifications(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, params HandleShowFeedsParams) {
	chain := alice.New(
		handlers.StoreFeedFilters(params),
		handlers.GenerateFeedsContent(s.DataAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.MarkFeeds(s.DataAPI()),
		handlers.RetrieveFeedFilters,
		handlers.GenerateFeedsContent(s.DataAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItems(res http.ResponseWriter, req *http.Request, params HandleShowItemsParams) {
	chain := alice.New(
		handlers.StoreItemFilters(params),
		handlers.GenerateItemsContent(s.DataAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleMarkItems(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.MarkItems(s.DataAPI()),
		handlers.RetrieveItemFilters,
		handlers.GenerateItemsContent(s.DataAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID) {
	chain := alice.New(
		handlers.GenerateItemArticle(s.DataAPI(), feedID, itemID),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

// HandleMarkItem marks a single item.
func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID, mark models.Mark) {
	marks := &models.MarkFeedItems{
		Feed:  feedID,
		Items: []models.ItemID{itemID},
		Mark:  mark,
	}
	// Mark item.
	if err := s.DataAPI().MarkItems(req.Context(), marks); err != nil {
		logging.FromContext(req.Context()).Error("Mark item failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed models.FeedID, item models.ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed models.FeedID, item models.ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}
