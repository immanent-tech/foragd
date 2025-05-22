// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/views"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// CheckRequiredFilters will ensure a request has the required filters set. If any required filters are missing,
// defaults will be substituted.
func CheckRequiredFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !(strings.HasPrefix(req.URL.Path, "/home") && req.Method == http.MethodGet) {
			next.ServeHTTP(res, req)
			return
		}

		ctx := req.Context()
		params := req.URL.Query()

		if !params.Has(string(models.ParamCount)) {
			params.Set(string(models.ParamCount), strconv.Itoa(models.DefaultCount))
		}

		if !params.Has(string(models.ParamView)) {
			params.Set(string(models.ParamView), string(models.DefaultView))
		}

		if !params.Has(string(models.ParamSortBy)) {
			params.Set(string(models.ParamSortBy), string(models.DefaultSortBy))
		}

		if !params.Has(string(models.ParamSortOrder)) {
			params.Set(string(models.ParamSortOrder), string(models.DefaultSortOrder))
		}

		req.URL.RawQuery = params.Encode()

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// GenerateFilters parses the request parameters, generates Filters from them and stores the filters in the session.
func GenerateFilters(session models.SessionAPI, params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Retrieve filters.
			filters, err := models.NewFiltersFromParams(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			ctx := models.FiltersToCtx(req.Context(), *filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func SavePageView(path string, params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Retrieve filters from params.
			filters, err := models.NewFiltersFromParams(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			// Create view.
			view := models.NewPageView(path, filters)
			// Save view in session.
			session := models.SessionFromCtx(req.Context())
			models.SaveViewInSession(req.Context(), session, view)
			// Save filters in context.
			ctx := models.FiltersToCtx(req.Context(), *filters)
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// MarkFeeds will mark the user's subscriptions that match the given feeds with the given mark.
func MarkFeeds(api DataAPI, mark models.Mark, feeds ...models.FeedID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get the user details.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			InternalServerError(res, req, ErrInvalidUser)
		}
		// Get the user subscriptions matching the feeds
		subscriptions := user.GetSubscriptions().FilterByFeedID(feeds...)
		// Mark the subscriptions.
		if err := api.MarkSubscriptions(req.Context(), mark, subscriptions.GetIDs()...); err != nil {
			InternalServerError(res, req, err)
			return
		}
		res.WriteHeader(http.StatusOK)
	})
}

// MarkItems handles marking items as read.
func MarkItems(api DataAPI, mark models.Mark, items ...models.ItemID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Mark the feeds.
		if err := api.MarkItems(req.Context(), mark, items...); err != nil {
			InternalServerError(res, req, err)
			return
		}
		res.WriteHeader(http.StatusOK)
	})
}

// DisplayFeeds handles displaying a list of feeds.
func DisplayFeeds(dataAPI DataAPI, sessionAPI models.SessionAPI, pagination models.Pagination) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		subscriptions, pagination, err := dataAPI.GetSubscriptions(req.Context(), pagination)
		if err != nil {
			InternalServerError(res, req, err)
			return
		}
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			backlink := models.GetBacklink(req.Context(), sessionAPI, models.FeedsRoute)
			PartialRender(
				home.HomeContent(home.GenerateFeedCards(req.Context(), subscriptions, pagination)...),
				layouts.Footer(
					partials.UpdateBacklink(backlink),
					home.UpdateFilters(models.FeedsRoute, subscriptions.GetCategoryCounts()),
					home.UpdateSorting(models.FeedsRoute),
					home.UpdateActions(
						home.AddSubscriptionAction(),
						home.ImportAction(),
						home.MarkAllFeedsAction(req.Context()),
					),
				),
				templates.SetPageTitle("Feeds"),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := home.BuildFeedsLayout(req.Context(), pagination, subscriptions)
			FullRender(layout.Title, templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}

// DisplayItems handles displaying a list of items.
func DisplayItems(dataAPI DataAPI, sessionAPI models.SessionAPI, pagination models.Pagination) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		items, pagination, err := dataAPI.GetItems(req.Context(), pagination)
		if err != nil {
			InternalServerError(res, req, err)
			return
		}
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			backlink := models.GetBacklink(req.Context(), sessionAPI, models.ItemsRoute)
			PartialRender(
				home.HomeContent(home.GenerateItemCards(req.Context(), items, pagination)...),
				layouts.Footer(
					partials.UpdateBacklink(backlink),
					home.UpdateFilters(models.ItemsRoute, items.GetCategoryCounts()),
					home.UpdateSorting(models.ItemsRoute),
					home.UpdateActions(
						home.AddSubscriptionAction(),
						home.ImportAction(),
						home.MarkAllItemsAction(req.Context(), items.GetFeedIDs()),
					),
				),
				templates.SetPageTitle("Items"),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := home.BuildItemsLayout(req.Context(), pagination, items)
			FullRender(layout.Title, templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}

// DisplayItem handles displaying an item as an article.
func DisplayItem(dataAPI DataAPI, sessionAPI models.SessionAPI, feedID models.FeedID, itemID models.ItemID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		item, found, err := dataAPI.GetItem(req.Context(), feedID, itemID)
		if err != nil || !found {
			InternalServerError(res, req, err)
			return
		}
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			backlink := models.GetBacklink(req.Context(), sessionAPI, "/home/"+item.GetFeedID()+"/"+item.GetID())
			PartialRender(
				home.HomeContent(home.GenerateArticle(item)),
				layouts.Footer(
					partials.UpdateBacklink(backlink),
				),
				templates.SetPageTitle(item.GetTitle()),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := home.BuildArticleLayout(req.Context(), item)
			FullRender(layout.Title, templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}

func DisplayHome(dataAPI DataAPI, sessionAPI models.SessionAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		content := views.GenerateHomePageContent(req.Context(), dataAPI.(*elastic.API))
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(

				layouts.Footer(),
				templates.SetPageTitle("Home"),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := &home.HomeLayout{
				Title:   "/home",
				Content: []templ.Component{content},
			}
			FullRender(layout.Title, templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}
