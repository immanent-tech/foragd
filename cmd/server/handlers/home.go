// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/session"
	"github.com/joshuar/go-feed-me/web/templates/layouts"
	"github.com/joshuar/go-feed-me/web/templates/layouts/home"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/partials/appbar"
)

func StoreFeedFilters(params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters := models.NewFilters()
			err := filters.Generate(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			session.StoreFeedFilters(req.Context(), filters)
			ctx := models.FiltersToCtx(req.Context(), *filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func RetrieveFeedFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := models.FiltersToCtx(req.Context(), session.GetFeedFilters(req.Context()))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

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
			footer := home.BuildListFooter(ctx, subscriptions.GetCategoryCounts())
			ctx = home.FooterToCtx(ctx, footer)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func StoreItemFilters(params any) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters := models.NewFilters()
			err := filters.Generate(params)
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			session.StoreItemFilters(req.Context(), filters)
			ctx := models.FiltersToCtx(req.Context(), *filters)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func RetrieveItemFilters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := models.FiltersToCtx(req.Context(), session.GetItemFilters(req.Context()))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func GenerateItemsContent(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get all items.
			items, pagination, err := api.GetItems(req.Context())
			if err != nil {
				InternalServerError(res, req, err)
				return
			}
			content := home.BuildItems(req.Context(), req.URL, pagination, items)
			ctx := home.ContentToCtx(req.Context(), content)
			footer := home.BuildListFooter(ctx, items.GetCategoryCounts())
			ctx = home.FooterToCtx(ctx, footer)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func DisplayHome(title string) http.Handler {
	return http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
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
				HandleHTMXResponse(resp, components...).ServeHTTP(res, req)
			} else {
				// Full render. Add the Appbar then build a full page layout to render.
				components = append(components, partials.CommandModal(), appbar.AppBar().Show(), home.NewContent(content...), footer.Show())
				fullPage := layouts.BuildPage(
					layouts.WithHeadOptions(title,
						layouts.WithPageDescription("Your home."),
						layouts.WithPageKeywords("feeds", "atom", "jsonfeed", "rss", "feed reader", "news", "current affairs"),
					),
					layouts.WithPageContent(components...),
				)
				HandleHTMXResponse(resp, fullPage.Show()).ServeHTTP(res, req)
			}
		})
}
