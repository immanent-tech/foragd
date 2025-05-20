// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"net/http"

	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
)

func (s Server) HandleHome(res http.ResponseWriter, req *http.Request) {
	handlers.InternalServerError(res, req, errors.New("not implemented"))
	// res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleHomeNotifications(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, params HandleShowFeedsParams) {
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.CheckRequiredFilters,
		handlers.GenerateFilters(s.SessionAPI(), params),
		handlers.SavePageView(models.FeedsRoute, params),
		handlers.GenerateFeedsContent(s.DataAPI(), pagination),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItems(res http.ResponseWriter, req *http.Request, params HandleShowItemsParams) {
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.CheckRequiredFilters,
		handlers.GenerateFilters(s.SessionAPI(), params),
		handlers.SavePageView(models.ItemsRoute, params),
		handlers.GenerateItemsContent(s.DataAPI(), s.SessionAPI(), pagination),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.GenerateItemArticle(s.DataAPI(), s.SessionAPI(), feedID, itemID),
	).Then(handlers.DisplayHome())
	chain.ServeHTTP(res, req)
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed models.FeedID, item models.ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed models.FeedID, item models.ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) MarkFeeds(res http.ResponseWriter, req *http.Request, mark Mark, params MarkFeedsParams) {
	var feedIDs []models.FeedID
	if params.Feeds != nil {
		feedIDs = *params.Feeds
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(params.RedirectOnSuccess),
	).Then(handlers.MarkFeeds(s.DataAPI(), mark, feedIDs...))
	chain.ServeHTTP(res, req)
}

func (s Server) MarkItems(res http.ResponseWriter, req *http.Request, mark Mark, params MarkItemsParams) {
	var itemIDs []models.FeedID
	if params.Items != nil {
		itemIDs = *params.Items
	}
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(params.RedirectOnSuccess),
	).Then(handlers.MarkItems(s.DataAPI(), mark, itemIDs...))
	chain.ServeHTTP(res, req)
}

func (s Server) MarkItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID, mark models.Mark, params MarkItemParams) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(params.RedirectOnSuccess),
	).Then(handlers.MarkItems(s.DataAPI(), mark, itemID))
	chain.ServeHTTP(res, req)
}
