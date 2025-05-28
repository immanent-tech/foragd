// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/web/templates"
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

// MarkItems handles marking items as read.
func MarkItems(api DataAPI, mark models.Mark, items ...models.ItemID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Mark the feeds.
		if err := api.MarkItems(req.Context(), mark, items...); err != nil {
			ProcessResponse(res, req, err)
			return
		}
		res.WriteHeader(http.StatusOK)
	})
}

func DisplaySubscriptions(dataAPI DataAPI, sessionAPI models.SessionAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		subscriptions, pagination, resp := dataAPI.GetSubscriptionsByID(req.Context(), models.FiltersFromCtx(req.Context()), pagination, subIDs...)
		if resp.IsError() {
			ProcessResponse(res, req, resp)
			return
		}
		pageTitle := "Subscriptions"
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				templ.Join(views.GenerateSubscriptionCards(req.Context(), subscriptions, pagination)...),
				partials.Footer(
					partials.UpdateBacklink(),
					partials.UpdateFilters(subscriptions.GetCategoryCounts()),
					partials.UpdateSorting(),
					partials.UpdateActions(
						views.AddSubscriptionAction(),
						views.ImportAction(),
						views.MarkAllSubscriptionsAction(req.Context()),
					),
				),
				templates.SetPageTitle(pageTitle),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := views.BuildSubscriptionsLayout(req.Context(), pagination, subscriptions)
			FullRender(pageTitle, templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}

func DisplayArticles(dataAPI DataAPI, sessionAPI models.SessionAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		articles, pagination, resp := dataAPI.GetArticlesBySubscription(req.Context(), models.FiltersFromCtx(req.Context()), pagination, subIDs...)
		if resp.IsError() {
			ProcessResponse(res, req, resp)
			return
		}
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				templ.Join(views.GenerateArticleCards(req.Context(), articles, pagination)...),
				partials.Footer(
					partials.UpdateBacklink(),
					partials.UpdateFilters(articles.GetItems().GetCategoryCounts()),
					partials.UpdateSorting(),
					partials.UpdateActions(
						views.AddSubscriptionAction(),
						views.ImportAction(),
						views.MarkAllArticlesAction(req.Context(), articles.GetSubscriptionIDs()...),
					),
				),
				templates.SetPageTitle("Items"),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			layout := views.BuildArticlesLayout(req.Context(), pagination, articles)
			FullRender("Items", templates.WithBody(layout)).ServeHTTP(res, req)
		}
	})
}

// DisplayArticle handles displaying an item as an article.
func DisplayArticle(dataAPI DataAPI, sessionAPI models.SessionAPI, itemID models.ItemID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		article, found, err := dataAPI.GetArticle(req.Context(), itemID)
		if err != nil || !found {
			spew.Dump(err, found)
			ProcessResponse(res, req, err)
			return
		}
		content := views.BuildArticleLayout(article)
		header := partials.Header(
			partials.DefaultHeaderStart(),
			partials.DefaultHeaderCenter(),
			partials.DefaultHeaderEnd(),
		)
		footer := partials.Footer(partials.UpdateBacklink())
		switch {
		case htmx.IsHTMX(req):
			// Update partial content for HTMX powered request.
			PartialRender(
				content,
				footer,
				templates.SetPageTitle(article.Item.GetTitle()),
			).ServeHTTP(res, req)
		default:
			// Generate full layout for non-HTMX powered request.
			FullRender(article.Item.GetTitle(),
				templates.WithBody(
					templates.NewBody(views.BuildArticleLayout(article),
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
