// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
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

// GenerateFeedsContent creates the content for displaying a list of feeds.
func GenerateFeedsContent(dataAPI DataAPI, pagination models.Pagination) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get feeds.
			subscriptions, pagination, err := dataAPI.GetSubscriptions(req.Context(), pagination)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			layout := home.BuildFeedsLayout(req.Context(), pagination, subscriptions)
			ctx := home.LayoutToCtx(req.Context(), layout)
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

// GenerateItemsContent creates the content for displaying a list of items.
func GenerateItemsContent(dataAPI DataAPI, sessionAPI models.SessionAPI, pagination models.Pagination) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			items, pagination, err := dataAPI.GetItems(req.Context(), pagination)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			layout := home.BuildItemsLayout(req.Context(), pagination, items)
			ctx := home.LayoutToCtx(req.Context(), layout)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateItemArticle creates the content for displaying an item.
func GenerateItemArticle(dataAPI DataAPI, sessionAPI models.SessionAPI, feedID models.FeedID, itemID models.ItemID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			item, found, err := dataAPI.GetItem(req.Context(), feedID, itemID)
			if err != nil || !found {
				InternalServerError(res, req, err)
				return
			}
			layout := home.BuildArticleLayout(req, item)
			ctx := home.LayoutToCtx(req.Context(), layout)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// DisplayHome displays a page under /home. It handles either partial or full rendering of the page, depending on
// whether the request is HTMX powered (partial) or not (full).
func DisplayHome() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		layout := home.LayoutFromCtx(req.Context())
		// Create a new response writer.
		ctx := context.WithValue(req.Context(), htmxRespCtxKey, htmx.NewResponse())
		// Decide whether we need to do a full or partial render based on whether
		// this is a htmx request or not.
		var handler http.Handler
		if htmx.IsHTMX(req) && req.Header.Get(htmx.HeaderHistoryRestoreRequest) != "true" {
			var parts []templ.Component
			// parts = append(parts, layout.Header)
			parts = append(parts, layout.Content...)
			parts = append(parts, layout.Footer)
			parts = append(parts, templates.SetPageTitle(layout.Title))
			handler = PartialRender(parts...)
		} else {
			handler = FullRender(layout.Title,
				templates.WithBody(layout),
			)
		}
		handler.ServeHTTP(res, req.WithContext(ctx))
	})
}
