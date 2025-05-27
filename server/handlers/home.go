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
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
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

// // GenerateFilters parses the request parameters, generates Filters from them and stores the filters in the session.
// func GenerateFilters(session models.SessionAPI, params any) func(next http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
// 			// Retrieve filters.
// 			filters, err := models.NewFiltersFromParams(params)
// 			if err != nil {
// 				InternalServerError(res, req, err)
// 				return
// 			}
// 			ctx := models.FiltersToCtx(req.Context(), *filters)
// 			next.ServeHTTP(res, req.WithContext(ctx))
// 		})
// 	}
// }

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
		subscriptions, pagination, err := dataAPI.GetSubscriptions(req.Context(), models.FiltersFromCtx(req.Context()), pagination)
		if err != nil {
			InternalServerError(res, req, err)
			return
		}
		pageTitle := "Feeds"
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				templ.Join(home.GenerateFeedCards(req.Context(), subscriptions, pagination)...),
				layouts.Footer(
					partials.UpdateBacklink(),
					home.UpdateFilters(subscriptions.GetCategoryCounts()),
					home.UpdateSorting(),
					home.UpdateActions(
						home.AddSubscriptionAction(),
						home.ImportAction(),
						home.MarkAllFeedsAction(req.Context()),
					),
				),
				templates.SetPageTitle(pageTitle),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := home.BuildFeedsLayout(req.Context(), pagination, subscriptions)
			FullRender(pageTitle, templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}

// DisplayItems handles displaying a list of items.
func DisplayItems(dataAPI DataAPI, sessionAPI models.SessionAPI, pagination models.Pagination) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		items, pagination, err := dataAPI.GetItems(req.Context(), models.FiltersFromCtx(req.Context()), pagination)
		if err != nil {
			InternalServerError(res, req, err)
			return
		}
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				templ.Join(home.GenerateItemCards(req.Context(), items, pagination)...),
				layouts.Footer(
					partials.UpdateBacklink(),
					home.UpdateFilters(items.GetCategoryCounts()),
					home.UpdateSorting(),
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
			FullRender("Items", templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}

// DisplayItem handles displaying an item as an article.
func DisplayItem(dataAPI DataAPI, sessionAPI models.SessionAPI, itemID models.ItemID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		item, found, err := dataAPI.GetItem(req.Context(), itemID)
		if err != nil || !found {
			InternalServerError(res, req, err)
			return
		}
		content := home.GenerateArticle(item)
		header := partials.Header(
			partials.DefaultHeaderStart(),
			partials.DefaultHeaderCenter(),
			partials.DefaultHeaderEnd(),
		)
		footer := layouts.Footer(partials.UpdateBacklink())
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				content,
				footer,
				templates.SetPageTitle(item.GetTitle()),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			FullRender(item.GetTitle(),
				templates.WithBody(
					templates.NewBody(home.GenerateArticle(item),
						templates.WithBodyHeader(header),
						templates.WithBodyFooter(footer),
					),
				),
			).ServeHTTP(res, req)
		}
	})
}

func DisplayHome(dataAPI DataAPI, sessionAPI models.SessionAPI) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		content := views.GenerateHomePageContent(req.Context(), dataAPI.(*elastic.API))
		header := partials.Header(
			partials.DefaultHeaderStart(),
			partials.DefaultHeaderCenter(),
			partials.DefaultHeaderEnd(),
		)
		footer := partials.Footer()
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				content,
				header,
				footer,
				templates.SetPageTitle("Home"),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			FullRender("Home", templates.WithBody(
				templates.NewBody(content,
					templates.WithBodyHeader(header),
					templates.WithBodyFooter(footer),
				),
			),
			).ServeHTTP(res, req)
		}
	})
}
