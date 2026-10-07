// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/components"
	"github.com/immanent-tech/foragd/web/templates/element"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

type MapArticles struct {
	title    templates.PageTitle
	template templ.Component
	svc      pageServices
}

func (p *MapArticles) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			p.svc.appCfg,
			p.svc.sessionMgr,
			p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

func (p *MapArticles) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	if req.Header.Get(models.ActionHeader) != "update-filters" {
		templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	} else {
		res.Header().Set(htmx.HeaderRetarget, "#map-markers")
		templ.Handler(p.template, templ.WithFragments(components.MapMarkersFragment)).ServeHTTP(res, req)
	}
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(templates.NavMap, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(templates.NavMap, element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func (m *Manager) HandleMap(itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters := ListFiltersFromCtx(req.Context())

		// Get articles matching filters.
		articles, subscriptions, next, err := itemSvc.FilterGeoArticles(req.Context(), filters)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			m.HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("filter articles: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// If the results are from a single subscription, save that subscription's details
		var subscription *models.Subscription
		if len(subscriptions) == 1 {
			subscription = subscriptions[0]
		}

		// Create response.
		response := &models.ListArticlesResponse{
			Subscription: subscription,
			Articles:     articles,
			Filters:      *filters,
		}
		// Update response filters.
		response.Filters.SearchAfter = next.SearchAfter
		response.Filters.UpTo = nil

		// If the list of articles is from a single subscription, update the page title to include the subscription
		// name.
		var titleSummary string
		if len(articles) > 0 && subscription != nil {
			titleSummary = subscription.GetTitle() + " | Articles"
		} else {
			titleSummary = "Articles"
		}
		title := templates.PageTitle{
			Summary:     titleSummary,
			Description: string(response.Filters.GetView()) + " | " + response.Filters.GetSort().String(),
		}

		// Choose rendering method based on method.
		switch req.Method {
		case http.MethodGet:
			// GET: render full page.
			RenderInternalPage(&MapArticles{
				title:    title,
				template: templates.MapArticles(response),
				svc:      m.NewPageServices(),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			// POST: render cards only.
			RenderPartial(&MapArticles{
				title:    title,
				template: templates.MapArticles(response),
			}).ServeHTTP(res, req)
			// Update category filters.
			RenderPartial(&PartialTemplate{
				template: templates.UpdateListCategoryFilters(
					"/articles",
					response.Filters,
					response.Articles.GetCategoryCounts().GetCategories(),
				),
			}).ServeHTTP(res, req)
		}
	}
}

func (m *Manager) HandleMapUpdates(itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters := ListFiltersFromCtx(req.Context())

		// Don't bother calculating updates if user is not viewing unread items.
		if filters.GetView() != models.ViewUnread {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Count items matching.
		updateCount, err := itemSvc.CountGeoArticles(req.Context(), filters)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get updates.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// If updates found, render a notification.
		if updateCount > 0 {
			// Reset from count.
			if filters.From != nil {
				filters.From = nil
			}
			RenderPartial(&PartialTemplate{template: partials.UpdatesToast(
				element.WithHXMethod(http.MethodGet, "/map"),
				element.WithHXTarget(templates.ContentID.Target()),
				element.WithHXSwap("innerMorph scroll:top transition:true"),
				element.WithHXValues(filters),
			)}).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}
}
