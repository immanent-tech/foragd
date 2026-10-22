// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/server/forms"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListCategories handles returning a list of categories that can be used for filtering subscriptions or articles.
func (m *Manager) ListCategories(subSvc SubscriptionsService, itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters := ListFiltersFromCtx(req.Context())
		switch {
		case strings.HasPrefix(req.URL.Path, "/subscriptions"):
			// Parse the list of displayed subscriptions.
			request, err := forms.DecodeForm[*models.ListSubscriptionCategoriesRequest](req)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to parse list categories request",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusNoContent)
				return
			}

			if len(request.Subscriptions) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}

			// Get the categories for the subscriptions.
			counts := getCategorySuggestions(req.Context(), request.Subscriptions...)
			if len(counts) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}
			categories := counts.GetCategories()
			slices.Sort(categories)

			// Generate the categories list template.
			m.NewPartial(templates.UpdateListCategoryFilters(
				"/subscriptions",
				*filters,
				categories,
			)).Handle().ServeHTTP(res, req)
		case strings.HasPrefix(req.URL.Path, "/articles"):
			user := models.UserFromCtx(req.Context())
			if user == nil {
				slogctx.FromCtx(req.Context()).Warn("Could not get user data.")
				res.WriteHeader(http.StatusNoContent)
				return
			}

			// Get subscription details.
			subscriptions, err := subSvc.GetAllSubscriptions(req.Context())
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				slogctx.FromCtx(req.Context()).Warn("Could not list subscriptions.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusNoContent)
				return
			}
			// Filter subscriptions.
			subscriptions = subscriptions.
				FilterByView(filters.GetView()).
				FilterByIDs(filters.GetSubscriptions()...)

			if len(subscriptions) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}

			// Get categories for items.
			counts, err := itemSvc.GetTopCategoriesForItems(
				req.Context(),
				query.Bool(
					query.Filter(service.BuildItemQueries(user, filters.GetView(), subscriptions)...),
				),
			)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Could not get item categories.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusNoContent)
				return
			}
			if len(counts) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}

			categories := counts.GetCategories()
			slices.Sort(categories)

			// Generate the categories list template.
			m.NewPartial(templates.UpdateListCategoryFilters(
				"/articles",
				*filters,
				categories,
			)).Handle().ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusNotAcceptable)
		}
	}
}

func (m *Manager) ListLanguages(subSvc SubscriptionsService, itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters := ListFiltersFromCtx(req.Context())
		switch {
		case strings.HasPrefix(req.URL.Path, "/subscriptions"):
			// Parse the list of displayed subscriptions.
			request, err := forms.DecodeForm[*models.ListSubscriptionCategoriesRequest](req)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to parse list categories request",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusNoContent)
				return
			}

			if len(request.Subscriptions) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}

			allSubscriptions := models.SubscriptionsFromCtx(req.Context())
			if allSubscriptions == nil {
				res.WriteHeader(http.StatusNoContent)
				return
			}
			subscriptions := allSubscriptions.FilterByIDs(request.Subscriptions...)

			counts := subscriptions.GetLanguageCounts()
			if len(counts) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}

			// Generate the categories list template.
			m.NewPartial(templates.UpdateListLanguageFilters(
				"/subscriptions",
				*filters,
				counts,
			)).Handle().ServeHTTP(res, req)
		case strings.HasPrefix(req.URL.Path, "/articles"):
			user := models.UserFromCtx(req.Context())
			if user == nil {
				slogctx.FromCtx(req.Context()).Warn("Could not get user data.")
				res.WriteHeader(http.StatusNoContent)
				return
			}

			// Get subscription details.
			subscriptions, err := subSvc.GetAllSubscriptions(req.Context())
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				slogctx.FromCtx(req.Context()).Warn("Could not list subscriptions.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusNoContent)
				return
			}
			// Filter subscriptions.
			subscriptions = subscriptions.
				FilterByView(filters.GetView()).
				FilterByIDs(filters.GetSubscriptions()...)

			if len(subscriptions) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}

			// Get categories for items.
			counts, err := itemSvc.GetTopLanguageCountsForItems(
				req.Context(),
				query.Bool(
					query.Filter(service.BuildItemQueries(user, filters.GetView(), subscriptions)...),
				),
			)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Could not get item categories.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusNoContent)
				return
			}
			if len(counts) == 0 {
				res.WriteHeader(http.StatusNoContent)
				return
			}

			// Generate the categories list template.
			m.NewPartial(templates.UpdateListLanguageFilters(
				"/articles",
				*filters,
				counts,
			)).Handle().ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusNotAcceptable)
		}
	}
}
