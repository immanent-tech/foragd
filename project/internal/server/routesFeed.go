// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials/content"
)

const (
	listFeedsPath = "/home/list/feeds"
	listItemsPath = "/home/list/items"
	articlePath   = "/home/article"
)

// FeedsHandler displays the home page with a list of feeds, optionally filtered
// by the given feed IDs and categories.
func (s Server) ListHandler(res http.ResponseWriter, req *http.Request, showType ListHandlerParamsListType, params ListHandlerParams) {
	var (
		page    templ.Component
		filters models.APISearchFilters
		cards   []templ.Component
		ctx     context.Context
	)

	logger := logging.NewHandlerLogger("ListHandler", req)

	// Bail if an invalid show parameter is requested.
	if chi.URLParam(req, "listType") != string(ListHandlerParamsListTypeFeeds) && chi.URLParam(req, "listType") != string(ListHandlerParamsListTypeItems) {
		// if showType != ListHandlerParamsListTypeFeeds && showType != ListHandlerParamsListTypeItems {
		slog.Info("here", slog.Any("type", showType))
		logger.Error("Bad request.", slog.Any("error", ErrInvalidQueryParams))
		res.WriteHeader(http.StatusBadRequest)

		return
	}

	if params.Feeds != nil {
		filters.FeedIDs = *params.Feeds
	}

	if params.Categories != nil {
		filters.Categories = *params.Categories
	}

	if params.Pagination != nil {
		if pagination, err := url.QueryUnescape(*params.Pagination); err != nil {
			logger.Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		} else {
			filters.Pagination = []byte(pagination)
		}
	}

	// Get all subscribed feeds.
	feeds, err := models.GetSubcribedFeeds(req.Context(), s.API.elastic, s.API.pg, filters)
	if err != nil {
		logger.Error("Cannot display content.", slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}

	switch chi.URLParam(req, "listType") {
	case string(ListHandlerParamsListTypeFeeds):
		ctx = content.NavigationToCtx(req.Context(), content.NavigationLinks{
			Backlink: stripBacklink(req.URL),
		})

		for _, feed := range feeds {
			unread := feed.GetUnreadCount(req.Context(), s.API.elastic)
			cards = append(cards, content.NewFeedCard(ctx, &feed, unread))
		}
	case string(ListHandlerParamsListTypeItems):
		items, pagination, err := s.API.elastic.GetItems(req.Context(), filters)
		if err != nil {
			logger.Warn("Could not retrieve items.", slog.Any("error", err))
		}

		ctx = content.NavigationToCtx(req.Context(), content.NavigationLinks{
			Parent:     generateParent(listFeedsPath, params.Backlink),
			Backlink:   stripBacklink(req.URL),
			Pagination: generatePagination(stripBacklink(req.URL), pagination),
		})

		for _, item := range items {
			cards = append(cards, content.NewItemCard(ctx, &item))
		}
	}

	if !htmx.IsHTMX(req) {
		// Full page when not htmx.
		page = layouts.NewPage("Go Feed Me - Home",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(content.ShowCards("feeds", cards...)))).Show()
	} else {
		// Partial content otherwise.
		page = content.ShowCards(string(showType), cards...)
	}

	if err := htmx.NewResponse().RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) ArticleHandler(res http.ResponseWriter, req *http.Request, feedID, itemID string, params ArticleHandlerParams) {
	var page templ.Component

	logger := logging.NewHandlerLogger("ArticleHandler", req)

	if feedID == "" || itemID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrMissingQueryParams))
		http.Error(res, "Invalid request.", http.StatusBadRequest)

		return
	}

	ctx := req.Context()

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		Parent: generateParent(listItemsPath, params.Backlink),
	})

	item, err := models.GetItem(ctx, s.API.pg, s.API.elastic, feedID, itemID)
	if err != nil {
		logger.Error("Could not get item.", slog.Any("error", err))
		http.Error(res, "Not found!.", http.StatusNotFound)

		return
	}

	if !htmx.IsHTMX(req) {
		page = layouts.NewPage("Go Feed Me - Home",
			layouts.WithPageDescription("Your home."),
			layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			layouts.WithPageContent(layouts.HomeLayout(content.ShowArticle(item)))).Show()
	} else {
		page = content.ShowArticle(item)
	}

	if err := htmx.NewResponse().RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
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
		if parent, err = url.Parse(fallback); err != nil {
			slog.Warn("Failed to generate fallback parent link.", slog.Any("error", err))
		}
	}

	return parent
}

func generatePagination(backlink *url.URL, pagination []byte) *url.URL {
	q := backlink.Query()
	q.Add("pagination", url.QueryEscape(string(pagination)))
	backlink.RawQuery = q.Encode()

	return backlink
}

func stripBacklink(backlink *url.URL) *url.URL {
	q := backlink.Query()
	q.Del("backlink")
	q.Del("pagination")
	backlink.RawQuery = q.Encode()

	return backlink
}
