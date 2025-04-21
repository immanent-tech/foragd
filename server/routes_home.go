// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"log/slog"
	"net/http"

	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
)

func (s Server) HandleHome(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleHomeNotifications(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, params HandleShowFeedsParams) {
	chain := alice.New(
		handlers.RouteLogger("show_feeds"),
		handlers.CheckRequiredFilters,
		handlers.GenerateFilters(s.SessionAPI(), params),
		handlers.GenerateFeedsContent(s.DataAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger("mark_feeds"),
		handlers.MarkFeeds(s.DataAPI()),
		handlers.RetrieveFilters(s.SessionAPI()),
		handlers.GenerateFeedsContent(s.DataAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItems(res http.ResponseWriter, req *http.Request, params HandleShowItemsParams) {
	chain := alice.New(
		handlers.RouteLogger("show_items"),
		handlers.CheckRequiredFilters,
		handlers.GenerateFilters(s.SessionAPI(), params),
		handlers.GenerateItemsContent(s.DataAPI(), s.SessionAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleMarkItems(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger("mark_items"),
		handlers.MarkItems(s.DataAPI()),
		handlers.RetrieveFilters(s.SessionAPI()),
		handlers.GenerateItemsContent(s.DataAPI(), s.SessionAPI()),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID) {
	chain := alice.New(
		handlers.RouteLogger("show_item"),
		handlers.GenerateItemArticle(s.DataAPI(), s.SessionAPI(), feedID, itemID),
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
		slogctx.FromCtx(req.Context()).Error("Mark item failed.",
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
