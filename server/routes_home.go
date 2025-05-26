// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/handlers"
)

func (s Server) HandleHome(res http.ResponseWriter, req *http.Request) {
	view := models.NewPageView(chi.RouteContext(req.Context()).RoutePattern(), nil)
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SaveState(s.SessionAPI(), models.PageViewIDHome, view),
		// handlers.SaveLastPageView(s.SessionAPI()),
	).Then(handlers.DisplayHome(s.DataAPI(), s.SessionAPI()))
	chain.ServeHTTP(res, req)
}

func (s Server) HandleHomeNotifications(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, params HandleShowFeedsParams) {
	// Extract any pagination value.
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}
	// Extract the backlink or set a default.
	var backlink models.PageViewID
	if params.Backlink == "" {
		backlink = models.PageViewIDHome
	} else {
		backlink = params.Backlink
	}

	// Retrieve filters.
	filters, err := models.NewFiltersFromParams(params)
	if err != nil {
		handlers.InternalServerError(res, req, err)
		return
	}
	view := models.NewPageView(chi.RouteContext(req.Context()).RoutePattern(), filters)
	chain := alice.New(
		handlers.RouteLogger,
		handlers.CheckRequiredFilters,
		// handlers.GenerateFilters(s.SessionAPI(), params),
		handlers.SaveState(s.SessionAPI(), models.PageViewIDShowFeeds, view),
		handlers.SaveBacklink(backlink),
	// handlers.SaveLastPageView(s.SessionAPI()),
	).Then(handlers.DisplayFeeds(s.DataAPI(), s.SessionAPI(), pagination))
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItems(res http.ResponseWriter, req *http.Request, params HandleShowItemsParams) {
	var pagination models.Pagination
	if params.Pagination != nil {
		pagination = *params.Pagination
	}

	// Extract the backlink or set a default.
	var backlink models.PageViewID
	if params.Backlink == "" {
		backlink = models.PageViewIDHome
	} else {
		backlink = params.Backlink
	}

	// Retrieve filters.
	filters, err := models.NewFiltersFromParams(params)
	if err != nil {
		handlers.InternalServerError(res, req, err)
		return
	}
	view := models.NewPageView(chi.RouteContext(req.Context()).RoutePattern(), filters)
	chain := alice.New(
		handlers.RouteLogger,
		handlers.CheckRequiredFilters,
		// handlers.GenerateFilters(s.SessionAPI(), params),
		handlers.SaveState(s.SessionAPI(), models.PageViewIDShowItems, view),
		handlers.SaveBacklink(backlink),
	).Then(handlers.DisplayItems(s.DataAPI(), s.SessionAPI(), pagination))
	chain.ServeHTTP(res, req)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID, params HandleShowItemParams) {
	chain := alice.New(
		handlers.RouteLogger,
		// handlers.SaveLastPageView(s.SessionAPI()),
	).Then(handlers.DisplayItem(s.DataAPI(), s.SessionAPI(), feedID, itemID))
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
		handlers.SetupRedirect(params.Redirect),
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
		handlers.SetupRedirect(params.Redirect),
	).Then(handlers.MarkItems(s.DataAPI(), mark, itemIDs...))
	chain.ServeHTTP(res, req)
}

func (s Server) MarkItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID, mark models.Mark, params MarkItemParams) {
	chain := alice.New(
		handlers.RouteLogger,
		handlers.SetupRedirect(params.Redirect),
	).Then(handlers.MarkItems(s.DataAPI(), mark, itemID))
	chain.ServeHTTP(res, req)
}
