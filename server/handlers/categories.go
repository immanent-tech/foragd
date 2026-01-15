// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

func ListCategories() http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(func(res http.ResponseWriter, req *http.Request) {
			// Decode filters.
			filters, _, err := forms.DecodeForm[*models.ListFilters](req)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to parse list categories request",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusNoContent)
				return
			}

			var template templ.Component
			switch {
			case strings.HasPrefix(req.URL.Path, "/list/subscriptions"):
				// Parse the list of displayed subscriptions.
				request, valid, err := forms.DecodeForm[*models.ListSubscriptionCategoriesRequest](req)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to parse list categories request",
						slog.Any("error", err),
					)
					res.WriteHeader(http.StatusNoContent)
					return
				}
				if !valid {
					slogctx.FromCtx(req.Context()).Warn("Invalid list categories request",
						slog.Any("error", err),
					)
					res.WriteHeader(http.StatusNoContent)
					return
				}

				// Get the categories for the subscriptions.
				counts, err := models.GetCategoriesForSubscriptions(req.Context(), request.Subscriptions...)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not get all subscription categories.",
						slog.Any("error", err),
					)
					res.WriteHeader(http.StatusNoContent)
					return
				}

				// Generate the categories list template.
				template = templates.ListCategoryFilters(
					&models.CategoryFilters{
						Categories: counts,
						Path:       "/list/subscriptions",
						Filters:    *filters,
					},
				)
			case strings.HasPrefix(req.URL.Path, "/list/articles"):
				user, err := models.UserFromCtx(req.Context())
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not get user data.",
						slog.Any("error", err),
					)
					res.WriteHeader(http.StatusNoContent)
					return
				}
				// Get subscriptions based on filters.
				subscriptions, err := models.GetSubscriptions(req.Context(),
					models.GetSubscriptionsByIDs(filters.GetSubscriptions()...),
				)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not list subscriptions.",
						slog.Any("error", err),
					)
					res.WriteHeader(http.StatusNoContent)
					return
				}

				// Get categories for items.
				counts, err := models.GetTopCategoriesForItems(
					req.Context(),
					query.Bool(
						query.Should(models.BuildItemQueries(user, filters.GetView(), subscriptions)...),
					),
				)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not get item categories.",
						slog.Any("error", err),
					)
					res.WriteHeader(http.StatusNoContent)
					return
				}
				// Generate the categories list template.
				template = templates.ListCategoryFilters(
					&models.CategoryFilters{
						Categories: counts,
						Path:       "/list/articles",
						Filters:    *filters,
					},
				)
			}

			// Render the list of categories.
			renderPartial(
				templates.NewPartial(template),
			).ServeHTTP(res, req)
		}).ServeHTTP
}
