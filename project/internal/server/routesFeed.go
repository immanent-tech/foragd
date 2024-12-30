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
	"github.com/joshuar/go-feed-me/internal/server/session"
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
		// Save list feeds filters in session storage.
		session.SaveListFeedsFilters(req.Context(), filters)
		ctx = content.NavigationToCtx(req.Context(), content.NavigationLinks{
			Return: generateReturn(req.URL),
		})

		for _, feed := range feeds {
			unread := feed.GetUnreadCount(req.Context(), s.API.elastic)
			cards = append(cards, content.NewFeedCard(ctx, &feed, unread))
		}
	case string(ListHandlerParamsListTypeItems):
		// Save list items filters in session storage.
		session.SaveListItemsFilters(req.Context(), filters)

		items, pagination, err := s.API.elastic.GetItems(req.Context(), filters)
		if err != nil {
			logger.Warn("Could not retrieve items.", slog.Any("error", err))
		}

		ctx = content.NavigationToCtx(req.Context(), content.NavigationLinks{
			Parent:     generateBacklink(req.Context(), listFeedsPath),
			Return:     generateReturn(req.URL),
			Pagination: generatePagination(req.URL, pagination),
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

func (s Server) ListActionHandler(res http.ResponseWriter, req *http.Request, listType ListActionHandlerParamsListType, action ListActionHandlerParamsAction, params ListActionHandlerParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ArticleHandler(res http.ResponseWriter, req *http.Request, feedID, itemID string) {
	var page templ.Component

	logger := logging.NewHandlerLogger("ArticleHandler", req)

	if feedID == "" || itemID == "" {
		logger.Error("Invalid request.", slog.Any("error", ErrMissingQueryParams))
		http.Error(res, "Invalid request.", http.StatusBadRequest)

		return
	}

	ctx := req.Context()

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		Parent: generateBacklink(req.Context(), listItemsPath),
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

func (s Server) ArticleActionHandler(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID, action ArticleActionHandlerParamsAction) {
	res.WriteHeader(http.StatusNotImplemented)
}

func generateBacklink(ctx context.Context, basePath string) string {
	var (
		filters models.APISearchFilters
		err     error
	)

	switch basePath {
	case listFeedsPath:
		filters, err = session.LoadListFeedsFilters(ctx)
	case listItemsPath:
		filters, err = session.LoadListItemsFilters(ctx)
	}

	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate backlink.",
			slog.Any("error", err))
		return basePath
	}

	backlink, err := filters.GenerateURL(basePath)
	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate backlink.",
			slog.Any("error", err))
		return basePath
	}

	return backlink.String()
}

func generateReturn(currentURL *url.URL) string {
	q := currentURL.Query()
	q.Del("backlink")
	currentURL.RawQuery = q.Encode()

	return currentURL.String()
}

func generatePagination(currentURL *url.URL, pagination []byte) string {
	q := currentURL.Query()
	q.Del("pagination")
	q.Add("pagination", url.QueryEscape(string(pagination)))
	currentURL.RawQuery = q.Encode()

	return currentURL.Path
}
