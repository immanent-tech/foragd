// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

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
	feedsPage := layouts.NewPage("Go Feed Me - Feeds",
		layouts.WithPageDescription("Your feeds."),
		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		layouts.WithPageContent(layouts.HomeLayout(req.URL.Path+"/show")))

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, feedsPage.Show()); err != nil {
		logging.LogHandler("FeedsHandler", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Server) ShowFeeds(res http.ResponseWriter, req *http.Request, params ShowFeedsParams) {
	ctx := req.Context()

	var feedIDs []string
	var categories []string
	var paramsStr string

	if params.Feeds != nil {
		feedIDs = append(feedIDs, *params.Feeds...)
		paramsStr = "?feeds=" + strings.Join(feedIDs, ",")
	}

	if params.Categories != nil {
		categories = append(feedIDs, *params.Categories...)
		paramsStr = "&categories=" + strings.Join(categories, ",")
	}

	ctx = content.BackLinkToCtx(ctx, "/home/feeds/show"+paramsStr)

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
	if err := htmx.NewResponse().PushURL("/home/feeds"+paramsStr).RenderTempl(ctx, res, content.ShowContent("", content.ShowCards("feeds", feedCards...))); err != nil {
		logging.LogHandler("ShowFeeds", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func (s Server) ItemsHandler(res http.ResponseWriter, req *http.Request, params ItemsHandlerParams) {
	itemsPage := layouts.NewPage("Go Feed Me - Items",
		layouts.WithPageDescription("Your items."),
		layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
		layouts.WithPageContent(layouts.HomeLayout(req.URL.Path+"/show")))

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, itemsPage.Show()); err != nil {
		logging.LogHandler("ItemsHandler", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Server) ShowItems(res http.ResponseWriter, req *http.Request, params ShowItemsParams) {
	ctx := req.Context()

	backlink := req.Header.Get(HeaderBacklink)
	if backlink == "" {
		backlink = "/home/feeds/show"
	}

	var feedIDs []string
	var paramsStr string
	var categories []string

	if params.Feeds != nil {
		feedIDs = append(feedIDs, *params.Feeds...)
		paramsStr = "?feeds=" + strings.Join(feedIDs, ",")

		// ctx = templates.FeedsToContext(ctx, *params.Feeds)
	}
	if params.Categories != nil {
		categories = append(feedIDs, *params.Categories...)
		paramsStr = paramsStr + "&categories=" + strings.Join(categories, ",")
	}

	ctx = content.BackLinkToCtx(ctx, "/home/items/show"+paramsStr)

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
	items, err := s.API.elastic.GetFeedItems(ctx, subs...)
	if err != nil {
		logging.LogHandler("ShowItems", req).Error("Could not retrieve items.", slog.Any("error", err))
	}

	itemCards := make([]components.Card, len(items))

	// Create item cards.
	for i, item := range items {
		itemCards[i] = content.NewItemCard(ctx, &item)
	}

	// Render the list of feed cards.
	if err := htmx.NewResponse().PushURL("/home/items"+paramsStr).RenderTempl(ctx, res, content.ShowContent(backlink, content.ShowCards("items", itemCards...))); err != nil {
		logging.LogHandler("ShowItems", req).Error("Cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}

func (s Server) ArticleHandler(res http.ResponseWriter, req *http.Request, feedID, itemID string) {
	logger := s.Logger.With(slog.String("handler", "FeedItem"))

	ctx := req.Context()

	backlink := req.Header.Get(HeaderBacklink)
	if backlink == "" {
		backlink = "/home/items/show"
	}

	if feedID == "" || itemID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrFeedIDRequired))
		http.Error(res, "Invalid request.", http.StatusBadRequest)
		return
	}

	item, err := models.GetItem(ctx, s.API.pg, s.API.elastic, feedID, itemID)
	if err != nil {
		logger.Error("Could not get item.", slog.Any("error", err))
	}

	// res.WriteHeader(http.StatusNotImplemented)

	// Render the list of feed cards.
	if err := htmx.NewResponse().
		RenderTempl(ctx, res, content.ShowContent(backlink, content.ShowItem(item))); err != nil {
		logging.LogReq(req, http.StatusInternalServerError).Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)

		return
	}
}
