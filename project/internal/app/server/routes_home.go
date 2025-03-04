// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/davecgh/go-spew/spew"
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

func (s Server) HandlePaginateFeeds(res http.ResponseWriter, req *http.Request, params HandlePaginateFeedsParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request) {
	// Get the view filters for reloading the page.
	filters, err := models.FiltersFromQuery(models.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Get the mark request.
	marks, valid, err := forms.DecodeCustom(req, DecodeMarkFeeds)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Get the list of feeds to mark.
	feeds, err := marks.Feeds.Get()
	if err != nil {
		return
	}
	// Mark the feeds.
	if err := s.DataAPI().UserActionMarkFeeds(req.Context(), marks.Mark, feeds...); err != nil {
		return
	}
	// Reload the home page.
	feedsHandler(s.DataAPI(), res, req, *filters)
}

// DecodeMarkFeeds is a custom decoder function to decode a request for marking
// Feeds.
func DecodeMarkFeeds(values url.Values) (*MarkFeeds, error) {
	request := &MarkFeeds{}

	var (
		err     error
		feedIDs *models.FeedIDs
	)

	// Parse feeds param.
	err = runtime.BindQueryParameter("form", true, false, string(models.ParamFeeds), values, &feedIDs)
	if err != nil {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	if feedIDs != nil {
		request.Feeds = nullable.NewNullableWithValue(*feedIDs)
	}
	// Parse mark param.
	if request.Mark = models.Mark(values.Get(string(models.ParamMark))); request.Mark == "" {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	return request, nil
}

// Valid checks that the MarkFeeds object is valid.
func (f *MarkFeeds) Valid() bool {
	// Must have valid mark value.
	if !(f.Mark == models.MarkRead || f.Mark == models.MarkUnread) {
		return false
	}
	// Feeds must be specified.
	if !f.Feeds.IsSpecified() {
		return false
	}

	return true
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

func (s Server) HandlePaginateItems(res http.ResponseWriter, req *http.Request, params HandlePaginateItemsParams) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleMarkItems(res http.ResponseWriter, req *http.Request) {
	// Create filters for API requests.
	filters, err := models.FiltersFromQuery(models.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Get the mark request.
	marks, valid, err := forms.DecodeCustom(req, DecodeMarkItems)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Get the list of feeds to mark.
	items, err := marks.Items.Get()
	if err != nil {
		return
	}
	// Mark the feeds.
	if err := s.DataAPI().UserActionMarkItems(req.Context(), marks.Mark, items...); err != nil {
		return
	}
	// Reload page.
	itemsHandler(s.DataAPI(), res, req, *filters)
}

// DecodeMarkItems is a custom decoder function to decode a request for marking
// Items.
func DecodeMarkItems(values url.Values) (*MarkItems, error) {
	request := &MarkItems{}

	var (
		err     error
		itemIDs *models.ItemIDs
	)

	// Parse feeds param.
	err = runtime.BindQueryParameter("form", true, false, string(models.ParamItems), values, &itemIDs)
	if err != nil {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	if itemIDs != nil {
		request.Items = nullable.NewNullableWithValue(*itemIDs)
	}
	// Parse mark param.
	if request.Mark = models.Mark(values.Get(string(models.ParamMark))); request.Mark == "" {
		return nil, errors.Join(ErrParseMarkRequest, err)
	}

	return request, nil
}

// Valid checks that the MarkItems object is valid.
func (f *MarkItems) Valid() bool {
	// Must have valid mark value.
	if !(f.Mark == models.MarkRead || f.Mark == models.MarkUnread) {
		return false
	}
	// Items must be specified.
	if !f.Items.IsSpecified() {
		return false
	}

	return true
}

// itemsHandler handles a list of items.
func itemsHandler(api *elastic.Client, res http.ResponseWriter, req *http.Request, filters models.APIFilters) {
	// Get all items.
	items, pagination, err := api.UserActionGetItems(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}
	spew.Dump(pagination)

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
	for _, item := range items {
		itemCard, err := partials.NewItemCard(filters, item)
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
		home.WithPart(home.Header, home.ArticleHeader(details)),
		home.WithPart(home.Footer, home.FullFooter(session.GetRouteState(req.Context(), "/home/items"))),
	)

	if err := layout.Render(req, res); err != nil {
		logging.FromContext(req.Context()).Error("Show item failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// HandleMarkItem marks a single item.
func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feedID FeedID, itemID ItemID, mark Mark) {
	// Mark item.
	if err := s.DataAPI().UserActionMarkItems(req.Context(), mark, itemID); err != nil {
		logging.FromContext(req.Context()).Error("Mark item failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed FeedID, item ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func renderCards(res http.ResponseWriter, req *http.Request, cards []home.Content, categories []models.CategoryCount, filters *models.APIFilters, backPath string) {
	var (
		layout *home.LayoutProps
		title  string
	)

	switch req.URL.Path {
	case "/home/feeds":
		title = "Feeds"
	case "/home/items":
		title = "Items"
	}

	// Build page layout.
	if req.Method == http.MethodGet {
		layout = home.BuildLayout(
			home.WithContent(cards...),
			home.WithTitle(title),
			home.WithPart(home.Header, home.ListHeader(filters, categories, req.URL.Path)),
			home.WithPart(home.Footer, home.FullFooter(backPath)),
		)
	} else {
		layout = home.BuildLayout(
			home.WithContent(cards...),
			home.WithPart(home.Header, home.ListHeader(filters, categories, req.URL.Path)),
		)
	}
	// Render /home/feeds page.
	if err := layout.Render(req, res); err != nil {
		logging.FromContext(req.Context()).Error("Show cards failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}
