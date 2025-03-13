// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/elastic/go-elasticsearch/v8/typedapi"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
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

func (s Server) HandleHome(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleHomeNotifications(res http.ResponseWriter, req *http.Request) {
	// // Set headers for SSE
	res.Header().Set("Content-Type", "text/event-stream")
	res.WriteHeader(http.StatusOK)
	// res.Header().Set("Cache-Control", "no-cache")
	// res.Header().Set("Connection", "keep-alive")

	// Create a channel to send data
	dataCh := make(chan string)
	defer close(dataCh)

	// resp := htmx.NewResponse()

	go func() {
		s.Logger.Debug("Started listening for notifications...")
		for {
			select {
			case <-req.Context().Done():
				s.Logger.Debug("Stopped listening for notifications...")
				return
			case data := <-dataCh:
				s.Logger.Debug("notification!")
				var foo strings.Builder
				if err := home.Notify(data).Render(req.Context(), &foo); err != nil {
					// if err := resp.RenderTempl(req.Context(), res, home.Notify(data)); err != nil {
					s.Logger.Warn("error rendering", slog.Any("error", err))
				}
				fmt.Fprintf(res, "event: notification\ndata: %s\n\n", foo.String())
				res.(http.Flusher).Flush()

			}
		}
	}()

	// Get the view filters for reloading the page.
	filters, err := api.FiltersFromQuery(api.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	filters.SetSince("1m")

	// Simulate sending data periodically
	for {
		ctx, cancelFunc := context.WithCancel(req.Context())
		defer cancelFunc()
		// Get feeds.
		_, err := elastic.UserActionCountUnread(ctx, s.DataAPI().GetAPI(), *filters)
		if err != nil {
			logging.FromContext(ctx).Warn("Could not retrieve feeds.",
				slog.Any("error", err))
			// http.Error(res, err.Error(), http.StatusInternalServerError)

			// return
		}
		// if count > 0 {
		dataCh <- "Feeds updated at: " + time.Now().Format(time.TimeOnly)
		// }
		time.Sleep(time.Minute)
	}
}

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, params HandleShowFeedsParams) {
	// Save route state.
	session.SetRouteState(req.Context(), req.URL.Path, req.URL.String())

	// Create filters from params.
	filters, err := api.CreateFilters(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
	}

	displayFeeds(s.DataAPI().GetAPI(), res, req, *filters)
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
	if err := elastic.UserActionMarkFeeds(req.Context(), s.DataAPI().GetAPI(), marks.Mark, marks.Feeds...); err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
	// Reload the home page.
	displayFeeds(s.DataAPI().GetAPI(), res, req, *filters)
}

// Valid checks that the MarkFeeds object is valid.
func (f *MarkFeeds) Valid() bool {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false
	}
	return true
}

// displayFeeds handles showing a list of Feeds as cards with the given filters applied.
func displayFeeds(api *typedapi.API, res http.ResponseWriter, req *http.Request, filters api.Filters) {
	// Get feeds.
	feeds, err := elastic.UserActionGetFeeds(req.Context(), api, filters)
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
	categories, err := elastic.UserActionGetFeedCategories(req.Context(), api)
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

	displayItems(s.DataAPI().GetAPI(), res, req, *filters)
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
	if err := elastic.UserActionMarkItems(req.Context(), s.DataAPI().GetAPI(), marks.Mark, marks.Items...); err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
	// Reload page.
	displayItems(s.DataAPI().GetAPI(), res, req, *filters)
}

// Valid checks that the MarkItems object is valid.
func (f *MarkItems) Valid() bool {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false
	}
	return true
}

// displayItems handles showing list of Items as cards with the given filters applied.
func displayItems(api *typedapi.API, res http.ResponseWriter, req *http.Request, filters api.Filters) {
	// Get all items.
	items, pagination, err := elastic.UserActionGetItems(req.Context(), api, filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Could not retrieve items.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}
	// Get item categories.
	categories, err := elastic.UserActionGetItemCategories(req.Context(), api, filters)
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

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feed models.FeedID, item models.ItemID) {
	details, found, err := elastic.UserActionGetItem(req.Context(), s.DataAPI().GetAPI(), feed, item)
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
			home.BuildMainContent(details.GetTitle(), article),
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
func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID, mark api.Mark) {
	// Mark item.
	if err := elastic.UserActionMarkItems(req.Context(), s.DataAPI().GetAPI(), mark, itemID); err != nil {
		logging.FromContext(req.Context()).Error("Mark item failed.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (s Server) HandleSaveItem(res http.ResponseWriter, req *http.Request, feed models.FeedID, item models.ItemID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) HandleUnsaveItem(res http.ResponseWriter, req *http.Request, feed models.FeedID, item models.ItemID) {
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
					home.BuildMainContent(title, cards...),
				),
			)
		} else {
			layout = home.BuildLayout(
				home.WithParts(
					home.BuildHeaders(appbar.AppBar().Show(), home.ListHeader(filters, categories, req.URL.Path)),
					home.BuildMainContent(title, cards...),
					home.BuildFooter(backPath),
				),
			)
		}
	} else {
		layout = home.BuildLayout(
			home.WithParts(
				home.BuildHeaders(appbar.AppBar().Show(), home.ListHeader(filters, categories, req.URL.Path)),
				home.BuildMainContent(title, cards...),
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

func HomeStreamTest(res http.ResponseWriter, req *http.Request) {
	// Create a channel to send data to the template.
	data := make(chan string)
	// Run a background process that will take 10 seconds to complete.
	go func() {
		// Always remember to close the channel.
		defer close(data)
		for i := 0; i < 10; i++ {
			select {
			case <-req.Context().Done():
				// Quit early if the client is no longer connected.
				return
			case <-time.After(time.Second):
				// Send a new piece of data to the channel.
				data <- fmt.Sprintf("Part %d", i+1)
			}
		}
	}()

	// Pass the channel to the template.
	component := home.SSE(data)

	// Serve using the streaming mode of the handler.
	templ.Handler(component, templ.WithStreaming()).ServeHTTP(res, req)
}
