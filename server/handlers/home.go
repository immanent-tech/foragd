// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/forms"
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
func GenerateFilters(session SessionAPI, params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Retrieve filters.
			filters, err := models.NewFiltersFromParams(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			// Store filters in session.
			route := chi.RouteContext(req.Context()).RoutePattern()
			switch route {
			case models.FeedsRoute:
				session.Put(req.Context(), feedFiltersSessionKey, filters)
			case models.ItemsRoute:
				session.Put(req.Context(), itemFiltersSessionKey, filters)
			default:
				slogctx.FromCtx(req.Context()).Warn("Cannot generate filters, unknown route.",
					slog.String("route", route))
				return
			}
			ctx := models.FiltersToCtx(req.Context(), *filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func GenerateHomeNavigation(session SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var (
				route    string
				current  home.Navigation
				previous home.Navigation
			)
			if redirect := req.Header.Get(htmx.HeaderLocation); redirect != "" {
				route = redirect
			} else {
				route = chi.RouteContext(req.Context()).RoutePattern()
			}
			switch route {
			case models.FeedsRoute:
				filters := models.FiltersFromCtx(req.Context())
				current = home.NewNavigation(models.FeedsRoute, &filters)
				previous = home.NewNavigation("/home", nil)
			case models.ItemsRoute:
				currentFilters := models.FiltersFromCtx(req.Context())
				previousFilters, ok := session.Get(req.Context(), feedFiltersSessionKey).(models.Filters)
				if !ok {
					previousFilters = *models.NewFilters()
				}
				current = home.NewNavigation(models.ItemsRoute, &currentFilters)
				previous = home.NewNavigation(models.FeedsRoute, &previousFilters)
			}
			ctx := req.Context()
			slogctx.FromCtx(ctx).Debug("Saving current route.", slog.String("route", current.AsAction().String()))
			ctx = home.CurrentRouteToCtx(ctx, current)
			slogctx.FromCtx(ctx).Debug("Saving previous route.", slog.String("route", previous.AsAction().String()))
			ctx = home.PreviousRouteToCtx(ctx, previous)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// RetrieveFilters retrieves filters from the session and stores them in the request context.
func RetrieveFilters(session SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var (
				route   string
				ok      bool
				filters models.Filters
			)
			if redirect := req.Header.Get(htmx.HeaderLocation); redirect != "" {
				route = redirect
			} else {
				route = chi.RouteContext(req.Context()).RoutePattern()
			}
			switch route {
			case models.FeedsRoute:
				filters, ok = session.Get(req.Context(), feedFiltersSessionKey).(models.Filters)
			case models.ItemsRoute:
				filters, ok = session.Get(req.Context(), itemFiltersSessionKey).(models.Filters)
			}
			if !ok {
				filters = *models.NewFilters()
			}
			ctx := models.FiltersToCtx(req.Context(), filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateFeedsContent creates the content for displaying a list of feeds.
func GenerateFeedsContent(dataAPI DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get feeds.
			subscriptions, pagination, err := dataAPI.GetSubscriptions(req.Context())
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			layout := home.BuildFeedsLayout(req.Context(), pagination, subscriptions)
			ctx := templates.LayoutToCtx(req.Context(), layout)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// MarkFeeds will mark the user's subscriptions that match the given feeds with the given mark.
func MarkFeeds(api DataAPI, mark models.Mark, feeds ...models.FeedID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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
			req.Header.Add(htmx.HeaderLocation, models.FeedsRoute)
			next.ServeHTTP(res, req)
		})
	}
}

// MarkItems handles marking items as read.
func MarkItems(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get the mark request.
			marks, valid, err := forms.DecodeForm[*models.MarkFeedItems](req)
			if err != nil || !valid {
				InternalServerError(res, req, err)
				return
			}
			// Mark the feeds.
			if err := api.MarkItems(req.Context(), marks); err != nil {
				InternalServerError(res, req, err)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}

// GenerateItemsContent creates the content for displaying a list of items.
func GenerateItemsContent(dataAPI DataAPI, sessionAPI SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			items, pagination, err := dataAPI.GetItems(req.Context())
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			layout := home.BuildItemsLayout(req.Context(), pagination, items)
			ctx := templates.LayoutToCtx(req.Context(), layout)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateItemArticle creates the content for displaying an item.
func GenerateItemArticle(dataAPI DataAPI, sessionAPI SessionAPI, feedID models.FeedID, itemID models.ItemID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			item, found, err := dataAPI.GetItem(req.Context(), feedID, itemID)
			if err != nil || !found {
				InternalServerError(res, req, err)
				return
			}
			itemFilters, ok := sessionAPI.Get(req.Context(), itemFiltersSessionKey).(models.Filters)
			if !ok {
				itemFilters = *models.NewFilters()
			}
			backRoute := models.NewRoute(models.ItemsRoute, &itemFilters)
			layout := home.BuildArticleLayout(req, backRoute, item)
			ctx := templates.LayoutToCtx(req.Context(), layout)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SaveHomeHistory saves the request URL to the session as a navigation history marker.
func SaveHomeHistory(session SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			slog.Debug("Saving last visited page", slog.String("url", req.URL.String()))
			session.Put(req.Context(), HomeHistorySessionKey, req.URL.String())
			next.ServeHTTP(res, req)
		})
	}
}

// DisplayHome displays a page under /home. It handles either partial or full rendering of the page, depending on
// whether the request is HTMX powered (partial) or not (full).
func DisplayHome() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			layout := templates.LayoutFromCtx(req.Context())
			// Create a new response writer.
			resp := htmx.NewResponse()
			// Decide whether we need to do a full or partial render based on whether
			// this is a htmx request or not.
			if htmx.IsHTMX(req) {
				// Partial render. Update the page title and render all content combined.
				HTMXResponse(resp, layout.PartialRender()).ServeHTTP(res, req)
			} else {
				// Full render. Add the Appbar then build a full page layout to render.
				HTMXResponse(resp, layout.FullRender()).ServeHTTP(res, req)
			}
		})
}
