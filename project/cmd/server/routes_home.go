// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/elastic/go-elasticsearch/v8/typedapi"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/session"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
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

	displayFeeds(s.DataAPI().GetAPI(), res, req, filters)
}

// func (s Server) HandleFeedsPagination(res http.ResponseWriter, req *http.Request, pagination api.Pagination) {
// 	if !htmx.IsHTMX(req) {
// 		logging.FromContext(req.Context()).Warn("Invalid request: not htmx.")
// 		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
// 		return
// 	}

// 	// Get the view filters for reloading the page.
// 	filters, err := api.FiltersFromQuery(api.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
// 	if err != nil {
// 		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
// 		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
// 		return
// 	}
// 	filters.Pagination = &pagination

// 	// Get the list of feeds to display.
// 	feeds, err := getFeedList(req.Context(), s.DataAPI().GetAPI(), filters)
// 	if err != nil {
// 		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
// 		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
// 		return
// 	}

// 	// Create a new response writer.
// 	resp := htmx.NewResponse()

// 	// Render the list of feeds.
// 	if err := resp.RenderTempl(req.Context(), res, feeds.Show()); err != nil {
// 		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(templates.ErrRenderTempl, err)))
// 		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
// 		return
// 	}
// }

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
	displayFeeds(s.DataAPI().GetAPI(), res, req, filters)
}

// Valid checks that the MarkFeeds object is valid.
func (f *MarkFeeds) Valid() bool {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false
	}
	return true
}

// getFeedList retrieves a filtered list of Feeds as components that
// can be rendered on a page.
func getFeedList(ctx context.Context, esapi *typedapi.API, filters *api.Filters) (templates.Elements, error) {
	// Get feeds.
	feeds, err := elastic.UserActionGetFeeds(ctx, esapi, *filters)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve feeds: %w", err)
	}

	feedList := make(templates.Elements, 0, len(feeds))
	// Build feed cards.
	for _, feed := range feeds {
		var card *partials.Card

		card, err = partials.NewFeedCard(*filters, feed)
		if err != nil {
			logging.FromContext(ctx).Warn("Could not create card component for feed.",
				slog.String("feed_id", feed.GetID()),
				slog.Any("error", err))

			continue
		}

		feedList = append(feedList, card)
	}

	return feedList, nil
}

// getFeedCategoryCounts creates a list of categories that can be displayed as a
// filter option on a page.
func getFeedCategoryCounts(ctx context.Context, api *typedapi.API, filters *api.Filters) (templates.Element, error) {
	// Retrieve the feed categories and the unread counts.
	categoryCounts, err := elastic.UserActionGetFeedCategories(ctx, api)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve user categories: %w", err)
	}

	categories := home.BuildCategoryFilters(filters, categoryCounts, "/home/feeds")

	return categories, nil
}

// displayFeeds handles showing a list of Feeds as cards with the given filters applied.
func displayFeeds(api *typedapi.API, res http.ResponseWriter, req *http.Request, filters *api.Filters) {
	// Get the list of feeds to display.
	feeds, err := getFeedList(req.Context(), api, filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	var content templates.Elements
	if htmx.IsHTMX(req) {
		content = feeds
	} else {
		content = append(content, home.BuildMainContent(feeds...))
	}

	categories, err := getFeedCategoryCounts(req.Context(), api, filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
	views := home.BuildViewFilters(filters, "/home/feeds")

	header := home.BuildListHeader(home.BuildFiltersMenu(views, categories), home.BuildSortingMenu(filters, "/home/feeds"))
	footer := home.BuildFooter("/home")

	displayHome(res, req, "Feeds", header, content, footer)
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

	displayItems(s.DataAPI().GetAPI(), res, req, filters)
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
	displayItems(s.DataAPI().GetAPI(), res, req, filters)
}

// Valid checks that the MarkItems object is valid.
func (f *MarkItems) Valid() bool {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false
	}
	return true
}

// getItemList retrieves items filtered by the given parameters as components
// that can be rendered on a page.
func getItemList(ctx context.Context, esapi *typedapi.API, path *url.URL, filters *api.Filters) (templates.Elements, error) {
	// Get all items.
	items, pagination, err := elastic.UserActionGetItems(ctx, esapi, *filters)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve items: %w", err)
	}

	itemList := make(templates.Elements, 0, filters.Count)
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard, err := partials.NewItemCard(*filters, item)
		if err != nil {
			logging.FromContext(ctx).Warn("Could not create card component for item.",
				slog.String("items_id", item.GetID()),
				slog.Any("error", err))

			continue
		}
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == filters.Count {
			itemCard.AddPagination(path, pagination)
		}
		// Append the card to the list of cards.
		itemList = append(itemList, itemCard)
	}

	return itemList, nil
}

func getItemCategoryCounts(ctx context.Context, esapi *typedapi.API, filters *api.Filters) (templates.Element, error) {
	// Get item categories.
	categoryCounts, err := elastic.UserActionGetItemCategories(ctx, esapi, *filters)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve item categories: %w", err)
	}

	categories := home.BuildCategoryFilters(filters, categoryCounts, "/home/items")

	return categories, nil
}

// displayItems handles showing list of Items as cards with the given filters applied.
func displayItems(esapi *typedapi.API, res http.ResponseWriter, req *http.Request, filters *api.Filters) {
	// Get the list of items to display.
	items, err := getItemList(req.Context(), esapi, req.URL, filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	var content templates.Elements
	if htmx.IsHTMX(req) {
		content = items
	} else {
		content = append(content, home.BuildMainContent(items...))
	}

	categories, err := getItemCategoryCounts(req.Context(), esapi, filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
	views := home.BuildViewFilters(filters, "/home/items")

	header := home.BuildListHeader(home.BuildFiltersMenu(views, categories), home.BuildSortingMenu(filters, "/home/items"))
	footer := home.BuildFooter("/home/feeds")

	displayHome(res, req, "Items", header, content, footer)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID) {
	details, found, err := elastic.UserActionGetItem(req.Context(), s.DataAPI().GetAPI(), feedID, itemID)
	if err != nil || !found {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	article := partials.NewArticle(&details)

	var content templates.Elements
	if htmx.IsHTMX(req) {
		content = append(content, article)
	} else {
		content = append(content, home.BuildMainContent(article))
	}

	header := home.BuildArticleHeader(&details)
	footer := home.BuildFooter("/home/items")

	displayHome(res, req, details.GetTitle(), header, content, footer)
}

// HandleMarkItem marks a single item.
func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, _ models.FeedID, itemID models.ItemID, mark api.Mark) {
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

// displayHome will render a /home page. It will select either a full (i.e.,
// non-htmx request) or partial (i.e., htmx request) render. The given title
// will be updated as well.
func displayHome(res http.ResponseWriter, req *http.Request, title string, content ...templates.Element) {
	// Generate the given content as templ.Components.
	parts := make([]templ.Component, 0, len(content))
	for _, c := range content {
		parts = append(parts, c.Show())
	}
	// Create a new response writer.
	resp := htmx.NewResponse()
	// Decide whether we need to do a full or partial render based on whether
	// this is a htmx request or not.
	if htmx.IsHTMX(req) {
		// Partial render. Update the page title and render all content combined.
		parts = append(parts, partials.NewTitle(title).Show())
		if err := resp.RenderTempl(req.Context(), res, templ.Join(parts...)); err != nil {
			logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
			http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
			return
		}
	} else {
		// Full render. Add the Appbar then build a full page layout to render.
		parts = slices.Insert(parts, 0, appbar.AppBar().Show())
		fullPage := layouts.BuildPage(
			layouts.WithHeadOptions(title,
				layouts.WithPageDescription("Your home."),
				layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
			),
			layouts.WithPageContent(parts...),
		)
		if err := resp.RenderTempl(req.Context(), res, fullPage.Show()); err != nil {
			logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
			http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
			return
		}
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
