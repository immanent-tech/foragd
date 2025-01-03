// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:lll
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/partials/content"
)

const (
	feedsListBasePath = "/home/feeds"
	itemsListBasePath = "/home/items"
	itemBasePath      = "/home/item"
)

// FeedsHandler displays the home page with a list of feeds, optionally filtered
// by the given feed IDs and categories.
func (s Server) GetListHandler(res http.ResponseWriter, req *http.Request, list GetListHandlerParamsListType, action GetListHandlerParamsGetAction, params GetListHandlerParams) {
	var (
		page    templ.Component
		filters models.APISearchFilters
		cards   []templ.Component
	)

	logger := logging.NewHandlerLogger("ListHandler", req)
	ctx := req.Context()

	logger.Debug("Handling Request.",
		slog.Any("action", action),
		slog.Any("list", list))

	// Bail if an invalid show parameter is requested.
	if list != GetListHandlerParamsListTypeFeeds && list != GetListHandlerParamsListTypeItems {
		logger.Error("Bad request.",
			slog.String("action", string(action)),
			slog.String("list", string(list)),
			slog.Any("error", ErrInvalidQueryParams))
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
	feeds, err := models.GetSubcribedFeeds(ctx, s.API.elastic, s.API.pg, filters)
	if err != nil {
		logger.Error("Cannot display content.", slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}

	switch list {
	case GetListHandlerParamsListTypeFeeds:
		// Save list feeds filters in session storage.
		session.SaveListFeedsFilters(ctx, filters)

		ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
			ActionBasePath:      feedsListBasePath,
			ChildActionBasePath: itemsListBasePath,
		})

		for _, feed := range feeds {
			unread := feed.GetUnreadCount(ctx, s.API.elastic)

			card, err := content.NewCard(ctx, &feed, unread)
			if err != nil {
				logger.Warn("Could not render item as card.", slog.Any("error", err))
				continue
			}

			cards = append(cards, card.Show())
		}
	case GetListHandlerParamsListTypeItems:
		// Save list items filters in session storage.
		session.SaveListItemsFilters(ctx, filters)

		items, pagination, err := s.API.elastic.GetItems(ctx, filters)
		if err != nil {
			logger.Warn("Could not retrieve items.", slog.Any("error", err))
		}

		ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
			Parent:              generateBacklink(ctx, feedsListBasePath),
			Pagination:          generatePagination(ctx, itemsListBasePath, pagination),
			ActionBasePath:      itemsListBasePath,
			ChildActionBasePath: itemBasePath,
		})

		for _, item := range items {
			card, err := content.NewCard(ctx, &item, 0)
			if err != nil {
				logger.Warn("Could not render item as card.", slog.Any("error", err))
				continue
			}

			cards = append(cards, card.Show())
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
		page = content.ShowCards(string(list), cards...)
	}

	if err := htmx.NewResponse().RenderTempl(ctx, res, page); err != nil {
		logger.Error("Cannot display content.", slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) PostListHandler(res http.ResponseWriter, req *http.Request, list PostListHandlerParamsListType, action PostListHandlerParamsPostAction, params PostListHandlerParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) GetItemHandler(res http.ResponseWriter, req *http.Request, action GetItemHandlerParamsGetAction, feedID FeedID, itemID ItemID) {
	var page templ.Component

	logger := logging.NewHandlerLogger("ArticleHandler", req)

	ctx := req.Context()

	ctx = content.NavigationToCtx(ctx, content.NavigationLinks{
		Parent:         generateBacklink(req.Context(), itemsListBasePath),
		ActionBasePath: itemBasePath,
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

func (s Server) PostItemHandler(res http.ResponseWriter, req *http.Request, action PostItemHandlerParamsPostAction, feed FeedID, item ItemID) {
	logger := logging.NewHandlerLogger("PostItemHandler", req)

	switch action {
	case PostItemHandlerParamsPostActionMarkRead:
		item := models.APIReadItem{
			ItemID: item,
			FeedID: feed,
		}
		if err := s.API.elastic.MarkItemsRead(req.Context(), item); err != nil {
			logger.Warn("Could not mark item as read.", slog.Any("error", err))
			return
		}
		if _, err := res.Write(nil); err != nil {
			logger.Error("Failed to write response.", slog.Any("error", err))
		}
	default:
		logger.Warn("Unimplmented.")
		res.WriteHeader(http.StatusNotImplemented)
	}
}

// generateBacklink creates a URL string that can be used for a back button on a
// page.
func generateBacklink(ctx context.Context, basePath string) string {
	var (
		filters models.APISearchFilters
		err     error
	)

	switch basePath {
	case feedsListBasePath:
		filters, err = session.LoadListFeedsFilters(ctx)
	case itemsListBasePath:
		filters, err = session.LoadListItemsFilters(ctx)
	}

	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate backlink.",
			slog.Any("error", err))
		return basePath + "/show"
	}

	backlink, err := filters.GenerateURL(basePath + "/show")
	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate backlink.",
			slog.Any("error", err))
		return basePath + "/show"
	}

	return backlink.String()
}

// generatePagination generates a URL string with an updated pagination value.
func generatePagination(ctx context.Context, basePath string, pagination []byte) string {
	var (
		filters models.APISearchFilters
		err     error
	)

	switch basePath {
	case feedsListBasePath:
		filters, err = session.LoadListFeedsFilters(ctx)
	case itemsListBasePath:
		filters, err = session.LoadListItemsFilters(ctx)
	}

	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate pagination link.",
			slog.Any("error", err))
		return basePath
	}

	paginationLink, err := filters.GenerateURL(basePath + "/show")
	if err != nil {
		logging.FromContext(ctx).Warn("Could not generate pagination link.",
			slog.Any("error", err))
		return basePath
	}

	q := paginationLink.Query()
	q.Del("pagination")
	q.Add("pagination", url.QueryEscape(string(pagination)))
	paginationLink.RawQuery = q.Encode()

	return paginationLink.String()
}
