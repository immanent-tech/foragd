// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

const (
	showFeedsPath       = "/home/feeds"
	markItemsReadPath   = "/home/markread/items"
	markItemsUnreadPath = "/home/markread/items"
	showItemPath        = "/home/show"
)

var ErrGeneratePageNavigationFailed = errors.New("error occurred while generating page navigation")

func HomeMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		params := req.URL.Query()
		if !params.Has("count") {
			params.Set("count", "10")
		}

		if !params.Has("view") {
			params.Set("view", "unread")
		}

		req.URL.RawQuery = params.Encode()

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func (s Server) HandleHome(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, reqParams HandleShowFeedsParams) {
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	// Save list feeds filters in session storage.
	// session.SaveListFeedsFilters(req.Context(), filters)

	feedCh, err := s.API.elastic.UserActionGetFeeds(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))

		// if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		// 	logging.FromContext(req.Context()).Error("Show feeds failed.",
		// 		slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
		// }

		return
	}

	// Retrieve the feed categories and the unread counts.
	categories, err := s.API.elastic.UserActionGetFeedCategories(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
	}

	var feeds []*templates.Component

	for feed := range feedCh {
		feedRoute := models.BuildRoute("/home/feed/" + feed.GetID() + "/items")

		component, err := templates.NewComponent(feed,
			templates.WithRoute("self", models.BuildRoute(req.URL.String())),
			templates.WithRoute("items", feedRoute),
			templates.DisplayAs(templates.FeedCard),
		)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		feeds = append(feeds, component)
	}

	layout := home.BuildLayout(
		home.WithContent(feeds...),
		home.WithHeader(
			home.FeedsHeader(
				partials.BuildCategoryFilters(reqParams.Categories, categories, req.URL.String()),
				partials.BuildViewFilter(reqParams.View, req.URL.String()))),
	)

	if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request, reqParams HandleMarkFeedsParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeedItems(res http.ResponseWriter, req *http.Request, feedID FeedID, reqParams HandleShowFeedItemsParams) {
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Add the feed ID.
	filters.FeedIDs = append(filters.FeedIDs, feedID)

	// Build a route for requests to perform actions against feeds.
	showItemsRoute := models.BuildRoute(req.URL)
	// // Get the feed details.
	// feed, err := s.API.elastic.GetFeedByID(req.Context(), feedID)
	// if err != nil {
	// 	logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", err))
	// 	http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

	// 	return
	// }

	// Save list items filters in session storage.
	// session.SaveListItemsFilters(req.Context(), filters)

	itemCh, _, err := s.API.elastic.UserActionGetItems(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	categoryCounts, err := s.API.elastic.UserActionGetItemCategories(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	items := make([]*templates.Component, 0, filters.Count)
	idx := 0

	for item := range itemCh {
		showArticleRoute := models.BuildRoute(filepath.Join(showItemPath, item.GetFeedID(), item.GetID()))

		component, err := templates.NewComponent(item,
			templates.DisplayAs(templates.ItemCard),
			templates.WithRoute("self", showItemsRoute),
			templates.WithRoute("article", showArticleRoute),
		)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for item.",
				slog.String("items_id", item.GetID()),
				slog.Any("error", err))

			continue
		}

		// if idx == len(itemCh)-1 && pagination != "" && len(itemCh) == filters.Count {
		// 	component.AddAttributes(templ.Attributes{
		// 		"hx-get":       filepath.Join("home", "show", "items") + "?pagination=" + pagination,
		// 		"hx-trigger":   "revealed",
		// 		"hx-swap":      "afterend",
		// 		"hx-push-url":  "false",
		// 		"hx-indicator": "#content-loading",
		// 	})
		// }

		items = append(items, component)
		idx++
	}

	layout := home.BuildLayout(
		home.WithContent(items...),
		home.WithHeader(
			home.FeedsHeader(
				partials.BuildCategoryFilters(reqParams.Categories, categoryCounts, req.URL.String()),
				partials.BuildViewFilter(reqParams.View, req.URL.String()))),
	)

	if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkFeedItems(res http.ResponseWriter, req *http.Request, feed FeedID, reqParams HandleMarkFeedItemsParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	layout := home.BuildLayout(
	// home.WithSideBar(
	// 	menu.WithID("drawer_menu"),
	// 	menu.WithExtraAttributes(templ.Attributes{
	// 		"hx-target":   "#drawer_menu",
	// 		"hx-swap-oob": "true",
	// 	}),
	// ),
	)

	details, found, err := s.API.elastic.UserActionGetItem(req.Context(), feed, item)
	if err != nil || !found {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
			logging.FromContext(req.Context()).Error("Show item failed.",
				slog.Any("error", err))
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	articleRoute := models.BuildRoute(req.URL)

	component, err := templates.NewComponent(details,
		templates.DisplayAs(templates.ItemArticle),
		templates.WithRoute("self", articleRoute),
	)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))

		if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
			logging.FromContext(req.Context()).Error("Show feeds failed.",
				slog.Any("error", err))
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}

		return
	}

	home.WithContent(component)(layout)

	if err := layout.Render(req.Context(), res, htmx.NewResponse(), htmx.IsHTMX(req)); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID, params HandleMarkItemParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}
