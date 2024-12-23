// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"

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
	var (
		page    templ.Component
		filters models.APISearchFilters
	)

	logger := logging.NewHandlerLogger("FeedsHandler", req)

	if params.Feeds != nil {
		filters.FeedIDs = *params.Feeds
	}

	if params.Categories != nil {
		filters.Categories = *params.Categories
	}

	if params.Pagination != nil {
		if pagination, err := url.QueryUnescape(*params.Pagination); err != nil {
			logger.Error("Could not unescape pagination parameter.", slog.Any("error", err))
		} else {
			filters.Pagination = []byte(pagination)
		}
	}

	ctx := content.NavigationToCtx(req.Context(), content.NavigationLinks{
		Backlink: stripBacklink(req.URL),
	})

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(ctx, s.API.elastic, s.API.pg, filters)
	if err != nil {
		logger.Error("Cannot display content.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}

	feedCards := make([]components.Card, len(feeds))

	// Generate cards for each feed.
	for i, feed := range feeds {
		feedCards[i] = content.NewFeedCard(ctx, &feed)
	}

	if !htmx.IsHTMX(req) {
		// Full page when not htmx.
		page = layouts.NewPage("Go Feed Me - Feeds",
			layouts.WithPageDescription("Your feeds."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(content.ShowCards("feeds", feedCards...)))).Show()
	} else {
		page = content.ShowCards("feeds", feedCards...)
	}

	if err := htmx.NewResponse().PushURL(req.URL.String()).RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Server) ItemsHandler(res http.ResponseWriter, req *http.Request, params ItemsHandlerParams) {
	var (
		page    templ.Component
		filters models.APISearchFilters
	)

	logger := logging.NewHandlerLogger("ItemsHandler", req)

	spew.Dump(req.URL)

	if params.Feeds != nil {
		filters.FeedIDs = *params.Feeds
	}

	if params.Categories != nil {
		filters.Categories = *params.Categories
	}

	if params.Pagination != nil {
		if pagination, err := url.QueryUnescape(*params.Pagination); err != nil {
			logger.Error("Could not unescape pagination parameter.", slog.Any("error", err))
		} else {
			filters.Pagination = []byte(pagination)
		}
	}

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), s.API.elastic, s.API.pg, filters)
	if err != nil {
		logger.Error("Could not retrieve feeds.", slog.Any("error", err))
	}

	subs := make([]string, len(feeds))

	// Get a list of the feed IDs.
	for i, feed := range feeds {
		subs[i] = feed.ID
	}
	// Get all feed items for all subscribed feeds.
	items, pagination, err := s.API.elastic.GetItems(req.Context(), filters)
	if err != nil {
		logger.Error("Could not retrieve items.", slog.Any("error", err))
	}

	ctx := content.NavigationToCtx(req.Context(), content.NavigationLinks{
		Parent:     generateParent("/home/feeds", params.Backlink),
		Backlink:   stripBacklink(req.URL),
		Pagination: url.QueryEscape(string(pagination)),
	})

	itemCards := make([]components.Card, len(items))

	// Create item cards.
	for i, item := range items {
		itemCards[i] = content.NewItemCard(ctx, &item)
	}

	if !htmx.IsHTMX(req) {
		page = layouts.NewPage("Go Feed Me - Items",
			layouts.WithPageDescription("Your items."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(content.ShowCards("items", itemCards...)))).Show()
	} else {
		page = content.ShowCards("items", itemCards...)
	}

	if err := htmx.NewResponse().PushURL(req.URL.String()).RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func (s Server) ArticleHandler(res http.ResponseWriter, req *http.Request, feedID, itemID string, params ArticleHandlerParams) {
	var page templ.Component

	logger := logging.NewHandlerLogger("ArticleHandler", req)

	if feedID == "" || itemID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrFeedIDRequired))
		http.Error(res, "Invalid request.", http.StatusBadRequest)

		return
	}

	ctx := req.Context()

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		Parent: generateParent("/home/items", params.Backlink),
	})

	item, err := models.GetItem(ctx, s.API.pg, s.API.elastic, feedID, itemID)
	if err != nil {
		logger.Error("Could not get item.", slog.Any("error", err))
	}

	if !htmx.IsHTMX(req) {
		page = layouts.NewPage("Go Feed Me - Items",
			layouts.WithPageDescription("Your items."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(content.ShowArticle(item)))).Show()
	} else {
		page = content.ShowArticle(item)
	}

	if err := htmx.NewResponse().PushURL(req.URL.String()).
		RenderTempl(ctx, res, page); err != nil {
		logger.Error("showFeedSummary: cannot render template.", slog.Any("error", err))
		res.WriteHeader(http.StatusInternalServerError)
	}
}

func generateParent(fallback string, source *Backlink) *url.URL {
	var (
		parent *url.URL
		err    error
	)

	if source != nil {
		parent, err = url.Parse(*source)
	}

	if err != nil || source == nil {
		parent, err = url.Parse(fallback)
		slog.Warn("Failed to generate fallback parent link.", slog.Any("error", err))
	}

	return parent
}

func stripBacklink(u *url.URL) *url.URL {
	q := u.Query()
	q.Del("backlink")
	u.RawQuery = q.Encode()

	return u
}
