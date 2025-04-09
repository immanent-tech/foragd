// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/cmd/server/handlers"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/session"
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
	res.WriteHeader(http.StatusNotImplemented)
}

// 	// // Set headers for SSE
// 	res.Header().Set("Content-Type", "text/event-stream")
// 	res.WriteHeader(http.StatusOK)
// 	// res.Header().Set("Cache-Control", "no-cache")
// 	// res.Header().Set("Connection", "keep-alive")

// 	// Create a channel to send data
// 	dataCh := make(chan string)
// 	defer close(dataCh)

// 	// resp := htmx.NewResponse()

// 	go func() {
// 		s.Log.Debug("Started listening for notifications...")
// 		for {
// 			select {
// 			case <-req.Context().Done():
// 				s.Log.Debug("Stopped listening for notifications...")
// 				return
// 			case data := <-dataCh:
// 				s.Log.Debug("notification!")
// 				var foo strings.Builder
// 				if err := home.Notify(data).Render(req.Context(), &foo); err != nil {
// 					// if err := resp.RenderTempl(req.Context(), res, home.Notify(data)); err != nil {
// 					s.Log.Warn("error rendering", slog.Any("error", err))
// 				}
// 				fmt.Fprintf(res, "event: notification\ndata: %s\n\n", foo.String())
// 				res.(http.Flusher).Flush()

// 			}
// 		}
// 	}()

// 	// Get the view filters for reloading the page.
// 	filters, err := models.FiltersFromQuery(models.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
// 	if err != nil {
// 		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
// 		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
// 		return
// 	}

// 	filters.SetSince("1m")

// 	// Simulate sending data periodically
// 	for {
// 		ctx, cancelFunc := context.WithCancel(req.Context())
// 		defer cancelFunc()
// 		// Get feeds.
// 		_, err := elastic.UserActionCountUnread(ctx, s.DataAPI().GetAPI(), *filters)
// 		if err != nil {
// 			logging.FromContext(ctx).Warn("Could not retrieve feeds.",
// 				slog.Any("error", err))
// 			// http.Error(res, err.Error(), http.StatusInternalServerError)

// 			// return
// 		}
// 		// if count > 0 {
// 		dataCh <- "Feeds updated at: " + time.Now().Format(time.TimeOnly)
// 		// }
// 		time.Sleep(time.Minute)
// 	}
// }

func (s Server) HandleShowFeeds(res http.ResponseWriter, req *http.Request, params HandleShowFeedsParams) {
	filters := models.NewFilters()
	err := filters.Generate(params)
	if err != nil {
		handlers.InternalServerError(res, req, err)
		return
	}
	session.StoreFeedFilters(req.Context(), filters)

	displayFeeds(s.DataAPI(), res, req, *filters)
}

// func (s Server) HandleFeedsPagination(res http.ResponseWriter, req *http.Request, pagination models.Pagination) {
// 	if !htmx.IsHTMX(req) {
// 		logging.FromContext(req.Context()).Warn("Invalid request: not htmx.")
// 		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
// 		return
// 	}

// 	// Get the view filters for reloading the page.
// 	filters, err := models.FiltersFromQuery(models.BuildRoute(session.GetRouteState(req.Context(), req.URL.Path)).GetParams())
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
	// Get the mark request.
	marks, valid, err := forms.DecodeForm[*models.MarkFeeds](req)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
	// Mark the feeds.
	if err := s.DataAPI().MarkSubscriptions(req.Context(), marks); err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	// Get the view filters for reloading the page.
	filters, err := session.GetFeedFilters(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	// Reload the home page.
	displayFeeds(s.DataAPI(), res, req, *filters)
}

// buildSubscriptionCards retrieves a filtered list of Feeds as components that
// can be rendered on a page.
func buildSubscriptionCards(filters models.Filters, subscriptions models.Subscriptions) (templates.Elements, error) {
	subscriptionCards := make(templates.Elements, 0, len(subscriptions))
	// Build feed cards.
	for subscription := range slices.Values(subscriptions) {
		subscriptionCards = append(subscriptionCards, home.BuildFeedCard(filters, subscription))
	}

	return subscriptionCards, nil
}

// displayFeeds handles showing a list of Feeds as cards with the given filters applied.
func displayFeeds(api DataAPI, res http.ResponseWriter, req *http.Request, filters models.Filters) {
	// Get feeds.
	subscriptions, err := api.GetSubscriptions(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("No subscriptions for user.")
	}

	// Get the list of feeds to display.
	feeds, err := buildSubscriptionCards(filters, subscriptions)
	if err != nil {
		logging.FromContext(req.Context()).Warn("No subscriptions for user.")
	}

	var content templates.Elements
	if htmx.IsHTMX(req) {
		content = feeds
	} else {
		content = append(content, home.BuildMainContent(feeds...))
	}

	viewFilters := home.BuildViewFilters(filters, "/home/feeds")
	categoryFilters := home.BuildCategoryFilters(filters, subscriptions.GetCategoryCounts(), "/home/feeds")

	// header := home.BuildListHeader(home.BuildFiltersMenu(views, categories), home.BuildSortingMenu(filters, "/home/feeds"))
	// footer := home.BuildFooter("/home")
	footer := home.BuildListFooter("/home",
		home.BuildFiltersMenu(viewFilters, categoryFilters),
		home.BuildSortingMenu(filters, "/home/feeds"),
		home.AddSubscriptionAction(), home.ImportAction(),
	)

	displayHome(res, req, "Feeds", content, footer)
}

func (s Server) HandleShowItems(res http.ResponseWriter, req *http.Request, params HandleShowItemsParams) {
	filters := models.NewFilters()
	err := filters.Generate(params)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Invalid parameters.", slog.Any("error", err))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	session.StoreItemFilters(req.Context(), filters)

	displayItems(s.DataAPI(), res, req, *filters)
}

func (s Server) HandleMarkItems(res http.ResponseWriter, req *http.Request) {
	// Get the mark request.
	marks, valid, err := forms.DecodeForm[*models.MarkFeedItems](req)
	if err != nil || !valid {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
	// Mark the feeds.
	if err := s.DataAPI().MarkItems(req.Context(), marks); err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}
	// Get the view filters for reloading the page.
	filters, err := session.GetItemFilters(req.Context())
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	// Reload page.
	displayItems(s.DataAPI(), res, req, *filters)
}

func buildItemCards(filters models.Filters, reqURL *url.URL, pagination models.Pagination, items models.Items) (templates.Elements, error) {
	itemCards := make(templates.Elements, 0, filters.Count)
	// Build item cards.
	for idx, item := range items {
		// Create a card for this item.
		itemCard := home.BuildItemCard(filters, item)
		// Add a pagination action to the last item.
		if idx == len(items)-1 && len(items) == filters.Count {
			itemCard.AddPagination(reqURL, pagination)
		}
		// Append the card to the list of cards.
		itemCards = append(itemCards, itemCard)
	}

	return itemCards, nil
}

// displayItems handles showing list of Items as cards with the given filters applied.
func displayItems(api DataAPI, res http.ResponseWriter, req *http.Request, filters models.Filters) {
	// Get all items.
	items, pagination, err := api.GetItems(req.Context(), filters)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	// Get the list of feeds to display.
	itemCards, err := buildItemCards(filters, req.URL, pagination, items)
	if err != nil {
		logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", errors.Join(ErrInvalidQueryParams, err)))
		http.Error(res, "fetch feed failed!", http.StatusInternalServerError)
		return
	}

	var content templates.Elements
	if htmx.IsHTMX(req) {
		content = itemCards
	} else {
		content = append(content, home.BuildMainContent(itemCards...))
	}

	viewFilters := home.BuildViewFilters(filters, "/home/items")
	categoryFilters := home.BuildCategoryFilters(filters, items.GetCategoryCounts(), "/home/items")

	// header := home.BuildListHeader(home.BuildFiltersMenu(views, categories), home.BuildSortingMenu(filters, "/home/items"))
	// footer := home.BuildFooter("/home/feeds")

	footer := home.BuildListFooter("/home/feeds",
		home.BuildFiltersMenu(viewFilters, categoryFilters),
		home.BuildSortingMenu(filters, "/home/items"),
	)

	displayHome(res, req, "Items", content, footer)
}

func (s Server) HandleShowItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID) {
	details, found, err := s.DataAPI().GetItem(req.Context(), feedID, itemID)
	if err != nil || !found {
		logging.FromContext(req.Context()).Warn("Could not retrieve item.",
			slog.Any("error", err))
		http.Error(res, err.Error(), http.StatusInternalServerError)

		return
	}

	article := home.NewArticle(details)

	var content templates.Elements
	if htmx.IsHTMX(req) {
		content = append(content, article)
	} else {
		content = append(content, home.BuildMainContent(article))
	}

	// header := home.BuildArticleHeader(&details)
	// footer := home.BuildFooter("/home/items")
	footer := home.BuildArticleFooter("/home/items", details)

	displayHome(res, req, details.GetTitle(), content, footer)
}

// HandleMarkItem marks a single item.
func (s Server) HandleMarkItem(res http.ResponseWriter, req *http.Request, feedID models.FeedID, itemID models.ItemID, mark models.Mark) {
	marks := &models.MarkFeedItems{
		Feed:  feedID,
		Items: []models.ItemID{itemID},
		Mark:  mark,
	}
	// Mark item.
	if err := s.DataAPI().MarkItems(req.Context(), marks); err != nil {
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
		parts = slices.Insert(parts, 0, partials.CommandModal(), appbar.AppBar().Show())
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
		for i := range 10 {
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
