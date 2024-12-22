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
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials/content"
)

var ErrFeedIDRequired = errors.New("feed ID is required")

// FeedsHandler displays the home page with a list of feeds, optionally filtered
// by the given feed IDs and categories.
func (s Server) FeedsHandler(res http.ResponseWriter, req *http.Request, params FeedsHandlerParams) {
	ctx := content.TriggerToCtx(req.Context(), req.URL.Path+"/show")

	feedsPage := layouts.NewPage("Go Feed Me - Feeds",
		layouts.WithPageDescription("Your feeds."),
		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		layouts.WithPageContent(layouts.HomeLayout(req.URL.Path+"/show")))

	if err := htmx.NewResponse().RenderTempl(ctx, res, feedsPage.Show()); err != nil {
		logging.LogHandler("FeedsHandler", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Server) ShowFeeds(res http.ResponseWriter, req *http.Request, params ShowFeedsParams) {
	ctx := req.Context()

	var feedIDs []string
	// var categories []string

	if params.Feeds != nil {
		feedIDs = append(feedIDs, *params.Feeds...)
	}

	// if params.Categories != nil {
	// 	categories = append(feedIDs, *params.Categories...)
	// }

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		Backlink: "/home/feeds/show?" + req.URL.RawQuery,
	})

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(ctx, s.API.elastic, s.API.pg, feedIDs...)
	if err != nil {
		logging.LogHandler("ShowFeeds", req).Error("Could not retrieve subscribed feeds.", slog.Any("error", err))
	}

	feedCards := make([]components.Card, len(feeds))

	// Generate cards for each feed.
	for i, feed := range feeds {
		feedCards[i] = content.NewFeedCard(ctx, &feed)
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().PushURL("/home/feeds?"+req.URL.RawQuery).RenderTempl(ctx, res, content.ShowCards("feeds", feedCards...)); err != nil {
		logging.LogHandler("ShowFeeds", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func (s Server) ItemsHandler(res http.ResponseWriter, req *http.Request, params ItemsHandlerParams) {
	ctx := content.TriggerToCtx(req.Context(), req.URL.Path+"/show")

	itemsPage := layouts.NewPage("Go Feed Me - Items",
		layouts.WithPageDescription("Your items."),
		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		layouts.WithPageContent(layouts.HomeLayout(req.URL.Path+"/show")))

	if err := htmx.NewResponse().RenderTempl(ctx, res, itemsPage.Show()); err != nil {
		logging.LogHandler("ItemsHandler", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Server) ShowItems(res http.ResponseWriter, req *http.Request, params ShowItemsParams) {
	ctx := req.Context()

	parent := req.Header.Get(content.HeaderBacklink)

	if parent == "" {
		parent = "/home/feeds/show"
	}

	var feedIDs []string
	// var categories []string

	if params.Feeds != nil {
		feedIDs = append(feedIDs, *params.Feeds...)
	}

	// if params.Categories != nil {
	// 	categories = append(feedIDs, *params.Categories...)
	// }

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(ctx, s.API.elastic, s.API.pg, feedIDs...)
	if err != nil {
		logging.LogHandler("ShowItems", req).Error("Could not retrieve feeds.", slog.Any("error", err))
	}

	subs := make([]string, len(feeds))

	// Get a list of the feed IDs.
	for i, feed := range feeds {
		subs[i] = feed.ID
	}
	// Get all feed items for all subscribed feeds.
	items, pagination, err := s.API.elastic.GetFeedItems(ctx, subs...)
	if err != nil {
		logging.LogHandler("ShowItems", req).Error("Could not retrieve items.", slog.Any("error", err))
	}

	itemCards := make([]components.Card, len(items))

	// Create item cards.
	for i, item := range items {
		itemCards[i] = content.NewItemCard(ctx, &item)
	}

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		Parent:     parent,
		Backlink:   "/home/items/show?" + req.URL.RawQuery,
		Pagination: string(pagination),
	})

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		PushURL("/home/items?"+req.URL.RawQuery).
		RenderTempl(ctx, res, content.ShowCards("items", itemCards...)); err != nil {
		logging.LogHandler("ShowItems", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func (s Server) ArticleHandler(res http.ResponseWriter, req *http.Request, feedID, itemID string) {
	logger := s.Logger.With(slog.String("handler", "FeedItem"))

	if feedID == "" || itemID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrFeedIDRequired))
		http.Error(res, "Invalid request.", http.StatusBadRequest)

		return
	}

	ctx := req.Context()

	parent := req.Header.Get(content.HeaderBacklink)

	if parent == "" {
		parent = "/home/items/show"
	}

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		Parent: parent,
	})

	item, err := models.GetItem(ctx, s.API.pg, s.API.elastic, feedID, itemID)
	if err != nil {
		logger.Error("Could not get item.", slog.Any("error", err))
	}

	// res.WriteHeader(http.StatusNotImplemented)

	// Render the list of feed cards.
	if err := htmx.NewResponse().PushURL(req.URL.Path).
		RenderTempl(ctx, res, content.ShowItem(item)); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}
