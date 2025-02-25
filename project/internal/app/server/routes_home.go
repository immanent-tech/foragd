// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

var ErrGeneratePageNavigationFailed = errors.New("error occurred while generating page navigation")

func SetCommonHomeFilters(next http.HandlerFunc) http.HandlerFunc {
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
	// Create filters from params.
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	// Save list feeds filters in session storage.
	session.SetRouteState(req.Context(), "/home/feeds", req.URL.String())

	feeds, err := s.getFeeds(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	// Retrieve the feed categories and the unread counts.
	categories, err := s.API.elastic.UserActionGetFeedCategories(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
	}

	// Build page layout.
	layout := home.BuildLayout(
		home.WithContent(feeds...),
		home.WithPart(home.Header,
			home.FullHeader(
				partials.BuildCategoryFilters(reqParams.Categories, categories, req.URL.String()),
				partials.BuildViewFilter(reqParams.View, req.URL.String()),
			),
		),
		home.WithPart(home.Footer, home.FullFooter("/home")),
	)

	// Render /home/feeds page.
	if err := layout.Render(req, res); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request, mark Mark, reqParams HandleMarkFeedsParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleShowFeedItems(res http.ResponseWriter, req *http.Request, feedID FeedID, reqParams HandleShowFeedItemsParams) {
	// Create filters for API requests.
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Add the feed ID.
	filters.FeedIDs = append(filters.FeedIDs, feedID)
	// Save list items filters in session storage.
	session.SetRouteState(req.Context(), "/home/feed/"+feedID, req.URL.String())

	// Get all items.
	itemCh, _, err := s.API.elastic.UserActionGetItems(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}
	// Get item categories.
	categoryCounts, err := s.API.elastic.UserActionGetItemCategories(req.Context(), *filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	items := make([]*templates.Component, 0, filters.Count)
	idx := 0
	// Build item cards.
	for item := range itemCh {
		component, err := templates.NewComponent(item,
			templates.DisplayAs(templates.ItemCard),
			templates.WithRoute(models.BuildRoute(
				"/home/feed/",
				models.WithSubPath(item.GetFeedID()),
				models.WithSubPath("item"),
				models.WithSubPath(item.GetID()),
			)),
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

	// Build page layout.
	layout := home.BuildLayout(
		home.WithContent(items...),
		home.WithPart(home.Header,
			home.FullHeader(
				partials.BuildCategoryFilters(reqParams.Categories, categoryCounts, req.URL.String()),
				partials.BuildViewFilter(reqParams.View, req.URL.String()),
			),
		),
		home.WithPart(home.Footer, home.FullFooter("/home/feeds")),
	)

	if err := layout.Render(req, res); err != nil {
		logging.FromContext(req.Context()).Error("Show feeds failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkFeedItems(res http.ResponseWriter, req *http.Request, feedID FeedID, mark Mark, reqParams HandleMarkFeedItemsParams) {
	if err := s.API.elastic.UserActionMarkFeeds(req.Context(), mark, feedID); err != nil {
		logging.FromContext(req.Context()).Error("Mark feed failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}

	route := models.BuildRoute(session.GetRouteState(req.Context(), "/home/feed/"+feedID))

	if route.GetViewParam() == models.ViewAll {
		feed, err := s.DataAPI().UserActionGetFeed(req.Context(), feedID)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))
		}

		component, err := templates.NewComponent(*feed,
			templates.WithRoute(route),
			templates.DisplayAs(templates.FeedCard),
		)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))
		}

		layout := home.BuildLayout(
			home.WithPart(home.Card, home.ShowFeedCard(component)),
		)

		if err := layout.PartRender(req.Context(), res, home.Card); err != nil {
			logging.FromContext(req.Context()).Error("Show feeds failed.",
				slog.Any("error", err))
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
	}
}

func (s Server) getFeeds(ctx context.Context, filters models.APIFilters) ([]*templates.Component, error) {
	// Get feeds.
	feedCh, err := s.API.elastic.UserActionGetFeeds(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve feeds: %w", err)
	}

	var feeds []*templates.Component
	// Build feed cards.
	for feed := range feedCh {
		component, err := templates.NewComponent(feed,
			templates.WithRoute(models.BuildRoute(
				"/home/feed/"+feed.GetID(),
				models.WithParams(
					models.WithCountParam(filters.Count),
					models.WithViewParam(filters.View),
				),
			)),
			templates.DisplayAs(templates.FeedCard),
		)
		if err != nil {
			logging.FromContext(ctx).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		feeds = append(feeds, component)
	}

	return feeds, nil
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	details, found, err := s.API.elastic.UserActionGetItem(req.Context(), feed, item)
	if err != nil || !found {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	articleRoute := models.BuildRoute(req.URL)

	component, err := templates.NewComponent(details,
		templates.DisplayAs(templates.ItemArticle),
		templates.WithRoute(articleRoute),
	)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	layout := home.BuildLayout(
		home.WithContent(component),
		home.WithPart(home.Footer, home.FullFooter(session.GetRouteState(req.Context(), "/home/feed/"+feed))),
	)

	if err := layout.Render(req, res); err != nil {
		logging.FromContext(req.Context()).Error("Show item failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID, mark Mark) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}
