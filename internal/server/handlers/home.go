// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/partials/appbar"
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

// StoreFeedFilters generates feed filters from the request params and then stores them in the session and request
// context.
func StoreFeedFilters(session SessionAPI, params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters := models.NewFilters()
			err := filters.Generate(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			session.Put(req.Context(), feedFiltersSessionKey, filters)
			ctx := models.FiltersToCtx(req.Context(), *filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// RetrieveFeedFilters retrieves feed filters from the session and stores them in the request context.
func RetrieveFeedFilters(session SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters, ok := session.Get(req.Context(), feedFiltersSessionKey).(models.Filters)
			if !ok {
				slogctx.FromCtx(req.Context()).Warn("No feed filters in session, using default filters.")
				filters = *models.NewFilters()
			}
			next.ServeHTTP(res, req.WithContext(models.FiltersToCtx(req.Context(), filters)))
		})
	}
}

// MarkFeeds handles marking feeds as read.
func MarkFeeds(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get the mark request.
			marks, valid, err := forms.DecodeForm[*models.MarkFeeds](req)
			if err != nil || !valid {
				InternalServerError(res, req, err)
				return
			}
			// Mark the feeds.
			if err := api.MarkSubscriptions(req.Context(), marks); err != nil {
				InternalServerError(res, req, err)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}

// GenerateFeedsContent creates the content for displaying a list of feeds.
func GenerateFeedsContent(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get feeds.
			subscriptions, err := api.GetSubscriptions(req.Context())
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			content := home.BuildFeeds(req.Context(), subscriptions)
			ctx := home.ContentToCtx(req.Context(), content)
			footer := home.BuildListFooter(ctx, home.BackButton(nil, "/home"), subscriptions.GetCategoryCounts())
			ctx = home.FooterToCtx(ctx, footer)
			ctx = home.TitleToCtx(ctx, "Feeds")
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// StoreItemFilters generates item filters from the request params and then stores them in the session and request
// context.
func StoreItemFilters(session SessionAPI, params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters := models.NewFilters()
			err := filters.Generate(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			session.Put(req.Context(), itemFiltersSessionKey, filters)
			next.ServeHTTP(res, req.WithContext(models.FiltersToCtx(req.Context(), *filters)))
		})
	}
}

// RetrieveItemFilters retrieves item filters from the session and stores them in the request context.
func RetrieveItemFilters(session SessionAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters, ok := session.Get(req.Context(), itemFiltersSessionKey).(models.Filters)
			if !ok {
				slogctx.FromCtx(req.Context()).Warn("No feed filters in session, using default filters.")
				filters = *models.NewFilters()
			}
			next.ServeHTTP(res, req.WithContext(models.FiltersToCtx(req.Context(), filters)))
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
			// Get all items.
			items, pagination, err := dataAPI.GetItems(req.Context())
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			content := home.BuildItems(req.Context(), req.URL, pagination, items)
			ctx := home.ContentToCtx(req.Context(), content)
			feedFilters, ok := sessionAPI.Get(req.Context(), feedFiltersSessionKey).(models.Filters)
			if !ok {
				feedFilters = *models.NewFilters()
			}
			back := home.BackButton(feedFilters.ToQueryParams(), models.FeedsRoute)
			footer := home.BuildListFooter(ctx, back, items.GetCategoryCounts())
			ctx = home.FooterToCtx(ctx, footer)
			ctx = home.TitleToCtx(ctx, "Items")
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
			content := []models.Template{home.BuildArticle(item)}
			ctx := home.ContentToCtx(req.Context(), content)
			itemFilters, ok := sessionAPI.Get(req.Context(), itemFiltersSessionKey).(models.Filters)
			if !ok {
				itemFilters = *models.NewFilters()
			}
			back := home.BackButton(itemFilters.ToQueryParams(), models.ItemsRoute)
			footer := home.BuildArticleFooter(item, back)
			ctx = home.FooterToCtx(ctx, footer)
			ctx = home.TitleToCtx(ctx, item.GetTitle())
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// DisplayHome displays a page under /home. It handles either partial or full rendering of the page, depending on
// whether the request is HTMX powered (partial) or not (full).
func DisplayHome() http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			title := home.TitleFromCtx(req.Context())
			content := home.ContentFromCtx(req.Context())
			footer := home.FooterFromCtx(req.Context())
			components := make([]templ.Component, 0, len(content))
			// Create a new response writer.
			resp := htmx.NewResponse()
			// Decide whether we need to do a full or partial render based on whether
			// this is a htmx request or not.
			if htmx.IsHTMX(req) {
				for c := range slices.Values(content) {
					components = append(components, c.Show())
				}
				// Partial render. Update the page title and render all content combined.
				components = append(components, footer.Show(), partials.NewTitle(title).Show())
				HTMXResponse(resp, components...).ServeHTTP(res, req)
			} else {
				// Full render. Add the Appbar then build a full page layout to render.
				components = append(components,
					partials.CommandModal(),
					appbar.AppBar().Show(),
					home.NewContent(content...),
					footer.Show())
				fullPage := layouts.BuildPage(
					layouts.WithHeadOptions(title,
						layouts.WithPageDescription("Your home."),
						layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
					),
					layouts.WithPageContent(components...),
				)
				HTMXResponse(resp, fullPage.Show()).ServeHTTP(res, req)
			}
		})
}
