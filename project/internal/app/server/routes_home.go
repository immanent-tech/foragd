// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/oapi-codegen/nullable"
	"github.com/oapi-codegen/runtime"

	"github.com/joshuar/go-feed-me/internal/app/server/forms"
	"github.com/joshuar/go-feed-me/internal/app/server/session"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

var (
	ErrGeneratePageNavigationFailed = errors.New("error occurred while generating page navigation")
	ErrParseMarkRequest             = errors.New("could not parse mark request")
)

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

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, params HandleShowFeedsParams) {
	// Save route state.
	session.SetRouteState(req.Context(), req.URL.Path, req.URL.String())

	// Create filters from params.
	filters, err := models.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	feedsHandler(s.DataAPI(), res, req, *filters)
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request) {
	// Create filters for API requests.
	filters, err := models.FiltersFromQuery(models.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}

	err = markHandler(s.DataAPI(), req)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}

	// Reload the home page.
	feedsHandler(s.DataAPI(), res, req, *filters)
}

// feedsHandler handles a list of feeds.
func feedsHandler(api *elastic.Client, res http.ResponseWriter, req *http.Request, filters models.APIFilters) {
	// Get feeds.
	feeds, err := api.UserActionGetFeeds(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	feedCards := make([]home.Content, 0, len(feeds))
	// Build feed cards.
	for _, feed := range feeds {
		var card *partials.Card

		card, err = partials.NewFeedCard(filters, feed)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		feedCards = append(feedCards, card)
	}

	// Retrieve the feed categories and the unread counts.
	categories, err := api.UserActionGetFeedCategories(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
	}

	renderCards(res, req, feedCards, categories, &filters, "/home")
}

func (s Server) HandleShowItems(res http.ResponseWriter, req *http.Request, params HandleShowItemsParams) {
	// Save list items filters in session storage.
	session.SetRouteState(req.Context(), req.URL.Path, req.URL.String())

	// Create filters for API requests.
	filters, err := models.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}

	itemsHandler(s.DataAPI(), res, req, *filters)
}

func (s Server) HandleMarkItems(res http.ResponseWriter, req *http.Request) {
	// Create filters for API requests.
	filters, err := models.FiltersFromQuery(models.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}

	err = markHandler(s.DataAPI(), req)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}

	// Reload page.
	itemsHandler(s.DataAPI(), res, req, *filters)
}

// itemsHandler handles a list of items.
func itemsHandler(api *elastic.Client, res http.ResponseWriter, req *http.Request, filters models.APIFilters) {
	// Get all items.
	itemCh, _, err := api.UserActionGetItems(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}
	// Get item categories.
	categories, err := api.UserActionGetItemCategories(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	itemCards := make([]home.Content, 0, filters.Count)
	idx := 0
	// Build item cards.
	for item := range itemCh {
		itemCard, err := partials.NewItemCard(filters, &item)
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

		itemCards = append(itemCards, itemCard)
		idx++
	}

	renderCards(res, req, itemCards, categories, &filters, "/home/feeds")
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	details, found, err := s.API.elastic.UserActionGetItem(req.Context(), feed, item)
	if err != nil || !found {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	article, err := partials.NewArticle(details)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	layout := home.BuildLayout(
		home.WithContent(article),
		home.WithPart(home.Footer, home.FullFooter(session.GetRouteState(req.Context(), "/home/items"))),
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

// DecodeMarkRequest is a custom decoder function to decode the mark request
// body.
func DecodeMarkRequest(values url.Values) (*MarkMultipleRequest, error) {
	request := &MarkMultipleRequest{}

	var (
		err        error
		feedIDs    *models.FeedIDs
		itemIDs    *models.ItemIDs
		categories *models.Categories
	)

	// Parse feeds param.
	err = runtime.BindQueryParameter("form", true, false, string(models.ParamFeeds), values, &feedIDs)
	if err != nil {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	if feedIDs != nil {
		request.Feeds = nullable.NewNullableWithValue(*feedIDs)
	}
	// Parse item params.
	err = runtime.BindQueryParameter("form", true, false, string(models.ParamItems), values, &itemIDs)
	if err != nil {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	if itemIDs != nil {
		request.Items = nullable.NewNullableWithValue(*itemIDs)
	}
	// Parse categories param.
	err = runtime.BindQueryParameter("form", true, false, string(models.ParamCategories), values, &categories)
	if err != nil {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	if categories != nil {
		request.Categories = categories
	}
	// Parse mark param.
	if request.Mark = models.Mark(values.Get(string(models.ParamMark))); request.Mark == "" {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	return request, nil
}

// Valid checks that the MarkMultipleRequest object is valid.
func (f *MarkMultipleRequest) Valid() bool {
	// Must have valid mark value.
	if !(f.Mark == models.MarkRead || f.Mark == models.MarkUnread) {
		return false
	}
	// Feeds or Items must be specified.
	if !f.Feeds.IsSpecified() && !f.Items.IsSpecified() {
		return false
	}

	return true
}

// markHandler handles parsing a mark request and performing the appropriate
// mark action on the requested feeds/items/categories.
func markHandler(api *elastic.Client, req *http.Request) error {
	// Decode mark parameters.
	marks, valid, err := forms.DecodeCustom(req, DecodeMarkRequest)
	if err != nil || !valid {
		return fmt.Errorf("unable to decode mark form data: %w", err)
	}

	// Mark any requested feeds.
	if marks.Feeds.IsSpecified() {
		// Get the list of feeds to mark.
		feeds, err := marks.Feeds.Get()
		if err != nil {
			return fmt.Errorf("unable to retrieve feeds to mark: %w", err)
		}
		// Mark the feeds.
		if err := api.UserActionMarkFeeds(req.Context(), marks.Mark, feeds...); err != nil {
			return fmt.Errorf("unable to mark feeds %s: %w", marks.Mark, err)
		}
	}

	// Mark any requested items.
	if marks.Items.IsSpecified() {
		// Get the list of feeds to mark.
		items, err := marks.Items.Get()
		if err != nil {
			return fmt.Errorf("unable to retrieve items to mark: %w", err)
		}
		// Mark feed.
		if err := api.UserActionMarkItems(req.Context(), marks.Mark, items...); err != nil {
			return fmt.Errorf("unable to mark items %s: %w", marks.Mark, err)
		}
	}

	return nil
}

func renderCards(res http.ResponseWriter, req *http.Request, cards []home.Content, categories []models.CategoryCount, filters *models.APIFilters, backPath string) {
	var layout *home.LayoutProps
	// Build page layout.
	if req.Method == http.MethodGet {
		layout = home.BuildLayout(
			home.WithContent(cards...),
			home.WithPart(home.Header,
				home.FullHeader(
					partials.BuildCategoryFilters(filters.GetCategories(), categories, req.URL.String()),
					partials.BuildViewFilter(filters.View, req.URL.String()),
				),
			),
			home.WithPart(home.Footer, home.FullFooter(backPath)),
		)
	} else {
		layout = home.BuildLayout(
			home.WithContent(cards...),
		)
	}
	// Render /home/feeds page.
	if err := layout.Render(req, res); err != nil {
		logging.FromContext(req.Context()).Error("Show cards failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}
