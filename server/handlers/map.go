// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type MapArticles struct {
	*Page
}

func (m *Manager) HandleMap(itemSvc ItemService) http.HandlerFunc {
	page := &MapArticles{
		Page: m.NewPage("/map",
			WithNavigation(templates.NavMap),
		),
	}
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
		page.template = templates.MapArticles(response)

		// If the list of articles is from a single subscription, update the page title to include the subscription
		// name.
		var titleSummary string
		if len(articles) > 0 && subscription != nil {
			titleSummary = subscription.GetTitle() + " | Articles"
		} else {
			titleSummary = "Articles"
		}
		page.Title.Summary = titleSummary
		page.Title.Description = string(response.Filters.GetView()) + " | " + response.Filters.GetSort().String()

		// Choose rendering method based on method.
		switch req.Method {
		case http.MethodGet:
			// GET: render full page.
			page.Handle().ServeHTTP(res, req)
		case http.MethodPost:
			// POST: render cards only.
			page.PartialResponse(res, req)
			// Update category filters.
			m.NewPartial(templates.UpdateListCategoryFilters(
				"/articles",
				response.Filters,
				response.Articles.GetCategoryCounts().GetCategories(),
			)).Handle().ServeHTTP(res, req)
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
			m.HandleUpdatesToast(res, req, "/map", filters)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}
}
