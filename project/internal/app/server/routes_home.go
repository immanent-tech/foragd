// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/joshuar/go-feed-me/internal/app/server/forms"
	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
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

	feedHandler(s.DataAPI(), res, req, *filters)
}

func (f *HandleMarkFeedsFormdataBody) Valid() bool {
	// Must have valid mark value.
	if !(f.Mark == models.MarkRead || f.Mark == models.MarkUnread) {
		return false
	}
	// Feeds must be specified.
	if f.Feeds == nil {
		// if !f.Feeds.IsSpecified() {
		return false
	}

	return true
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request, reqParams HandleMarkFeedsParams) {
	// Create filters for API requests.
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Decode mark parameters.
	marks, valid, err := forms.DecodeForm[*HandleMarkFeedsFormdataBody](req)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Error("Could not decode form data.",
			slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}
	// Get the list of feeds to mark.
	feeds := *marks.Feeds
	// feeds, err := marks.Feeds.Get()
	// if err != nil {
	// 	logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	// 	http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

	// 	return
	// }
	// Mark the feeds.
	if err := s.API.elastic.UserActionMarkFeeds(req.Context(), marks.Mark, feeds...); err != nil {
		logging.FromContext(req.Context()).Error("Mark feed failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	// Reload the home page.
	feedHandler(s.DataAPI(), res, req, *filters)
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

func (f *HandleMarkFeedItemsFormdataRequestBody) Valid() bool {
	// Must have valid mark value.
	if !(f.Mark == models.MarkRead || f.Mark == models.MarkUnread) {
		return false
	}
	// Feeds must be specified.
	if f.Items == nil {
		// if !f.Feeds.IsSpecified() {
		return false
	}

	return true
}

func (s Server) HandleMarkFeedItems(res http.ResponseWriter, req *http.Request, feedID FeedID, reqParams HandleMarkFeedItemsParams) {
	// Create filters for API requests.
	filters, err := models.CreateFilters(reqParams)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Decode mark parameters.
	marks, valid, err := forms.DecodeForm[*HandleMarkFeedItemsFormdataRequestBody](req)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Error("Could not decode submitted subscription request request.",
			slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}
	// Get the list of feeds to mark.
	// feeds, err := marks.Items.Get()
	items := *marks.Items
	// if err != nil {
	// 	logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	// 	http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

	// 	return
	// }
	// Mark feed.
	if err := s.API.elastic.UserActionMarkItems(req.Context(), marks.Mark, items...); err != nil {
		logging.FromContext(req.Context()).Error("Mark feed failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	// Reload home page.
	feedHandler(s.DataAPI(), res, req, *filters)
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

func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func feedHandler(api *elastic.Client, res http.ResponseWriter, req *http.Request, filters models.APIFilters) {
	// Save route state.
	session.SetRouteState(req.Context(), req.URL.Path, req.URL.String())

	// Get feeds.
	feedCh, err := api.UserActionGetFeeds(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
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
			logging.FromContext(req.Context()).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		feeds = append(feeds, component)
	}

	// Retrieve the feed categories and the unread counts.
	categories, err := api.UserActionGetFeedCategories(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
	}

	// Build page layout.
	layout := home.BuildLayout(
		home.WithContent(feeds...),
		home.WithPart(home.Header,
			home.FullHeader(
				partials.BuildCategoryFilters(&filters.Categories, categories, req.URL.String()),
				partials.BuildViewFilter(filters.View, req.URL.String()),
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
