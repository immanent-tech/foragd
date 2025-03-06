// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/session"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/partials/appbar"
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
	filters, err := api.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	feedsHandler(s.DataAPI(), res, req, *filters)
}

func (s Server) HandleMarkFeeds(res http.ResponseWriter, req *http.Request) {
	// Get the view filters for reloading the page.
	filters, err := api.FiltersFromQuery(api.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Get the mark request.
	marks, valid, err := forms.DecodeForm[*MarkFeeds](req)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Mark the feeds.
	if err := s.DataAPI().UserActionMarkFeeds(req.Context(), marks.Mark, marks.Feeds...); err != nil {
		return
	}
	// Reload the home page.
	feedsHandler(s.DataAPI(), res, req, *filters)
}

// Valid checks that the MarkFeeds object is valid.
func (f *MarkFeeds) Valid() bool {
	return validation.IsValid(f)
}

// feedsHandler handles a list of feeds.
func feedsHandler(api *elastic.Client, res http.ResponseWriter, req *http.Request, filters api.Filters) {
	// Get feeds.
	feeds, err := api.UserActionGetFeeds(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve feeds.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	feedCards := make([]home.Element, 0, len(feeds))
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
	filters, err := api.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}

	itemsHandler(s.DataAPI(), res, req, *filters)
}

func (s Server) HandleMarkItems(res http.ResponseWriter, req *http.Request) {
	// Create filters for API requests.
	filters, err := api.FiltersFromQuery(api.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Get the mark request.
	marks, valid, err := forms.DecodeForm[*MarkItems](req)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)

		return
	}
	// Mark the feeds.
	if err := s.DataAPI().UserActionMarkItems(req.Context(), marks.Mark, marks.Items...); err != nil {
		return
	}
	// Reload page.
	itemsHandler(s.DataAPI(), res, req, *filters)
}

// Valid checks that the MarkItems object is valid.
func (f *MarkItems) Valid() bool {
	return validation.IsValid(f)
}

// itemsHandler handles a list of items.
func itemsHandler(api *elastic.Client, res http.ResponseWriter, req *http.Request, filters api.Filters) {
	// Get all items.
	items, pagination, err := api.UserActionGetItems(req.Context(), filters)
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

	itemCards := make([]home.Element, 0, filters.Count)
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard, err := partials.NewItemCard(filters, item)
		if err != nil {
			logging.FromContext(req.Context()).Warn("Could not create card component for item.",
				slog.String("items_id", item.GetID()),
				slog.Any("error", err))

			continue
		}
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == filters.Count {
			itemCard.AddPagination(req.URL, pagination)
		}
		// Append the card to the list of cards.
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
		home.WithParts(
			home.BuildMainContent(article),
			home.BuildHeaders(appbar.AppBar().Show(), home.ArticleHeader(details)),
			home.BuildFooter("/home/items"),
		),
	)

	if err := layout.Render(res, req); err != nil {
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

func renderCards(res http.ResponseWriter, req *http.Request, cards []home.Element, categories []api.CategoryCount, filters *api.Filters, backPath string) {
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
		if filters.GetPagination() != "" {
			layout = home.BuildLayout(
				home.WithParts(
					home.BuildMainContent(cards...),
				),
			)
		} else {
			layout = home.BuildLayout(
				home.WithParts(
					home.BuildHeaders(appbar.AppBar().Show(), home.ListHeader(filters, categories, req.URL.Path)),
					home.BuildMainContent(cards...),
					home.BuildFooter(backPath),
				),
				home.WithTitle(title),
			)
		}
	} else {
		layout = home.BuildLayout(
			home.WithParts(
				home.BuildHeaders(appbar.AppBar().Show(), home.ListHeader(filters, categories, req.URL.Path)),
				home.BuildMainContent(cards...),
			),
		)
	}
	// Render /home/feeds page.
	if err := layout.Render(res, req); err != nil {
		logging.FromContext(req.Context()).Error("Show cards failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}
