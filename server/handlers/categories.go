// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListCategories handles returning a list of categories that can be used for filtering subscriptions or articles.
func ListCategories() http.HandlerFunc {
	return userContentHandlerChain.
		ThenFunc(func(res http.ResponseWriter, req *http.Request) {
			switch {
			case strings.HasPrefix(req.URL.Path, "/list/subscriptions"):
				filters := getListSubscriptionsFilters(req)
				// Parse the list of displayed subscriptions.
				request, valid, err := forms.DecodeForm[*models.ListSubscriptionCategoriesRequest](req)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to parse list categories request",
						slog.Any("error", err),
					)
					RenderPartial(&PartialTemplate{
						template: templates.CategoryFilters(&models.CategoryFilters{}),
					}).ServeHTTP(res, req)
					return
				}
				if !valid {
					slogctx.FromCtx(req.Context()).Warn("Invalid list categories request",
						slog.Any("error", err),
					)
					RenderPartial(&PartialTemplate{
						template: templates.CategoryFilters(&models.CategoryFilters{}),
					}).ServeHTTP(res, req)
					return
				}

				// Get the categories for the subscriptions.
				counts, err := service.GetCategoriesForSubscriptions(req.Context(), request.Subscriptions...)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not get all subscription categories.",
						slog.Any("error", err),
					)
					RenderPartial(&PartialTemplate{
						template: templates.CategoryFilters(&models.CategoryFilters{}),
					}).ServeHTTP(res, req)
					return
				}

				// Generate the categories list template.
				RenderPartial(&PartialTemplate{
					template: templates.CategoryFilters(
						&models.CategoryFilters{
							Categories: counts,
							Path:       "/list/subscriptions",
							Filters:    *filters,
						},
					),
				}).ServeHTTP(res, req)
			case strings.HasPrefix(req.URL.Path, "/list/articles"):
				articleFilters := getListArticleFilters(req)
				user := models.UserFromCtx(req.Context())
				if user == nil {
					slogctx.FromCtx(req.Context()).Warn("Could not get user data.")
					RenderPartial(&PartialTemplate{
						template: templates.CategoryFilters(&models.CategoryFilters{}),
					}).ServeHTTP(res, req)
					return
				}

				// Get subscription details.
				subscriptions, err := service.GetAllSubscriptions(req.Context())
				if err != nil && !errors.Is(err, models.ErrNotFound) {
					slogctx.FromCtx(req.Context()).Warn("Could not list subscriptions.",
						slog.Any("error", err),
					)
					RenderPartial(&PartialTemplate{
						template: templates.CategoryFilters(&models.CategoryFilters{}),
					}).ServeHTTP(res, req)
					return
				}
				// Update subscription dynamic info.
				if err = service.UpdateSubscriptionDynamicInfo(req.Context(), subscriptions); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to update subscription dynamic info.",
						slog.Any("error", err),
					)
					res.WriteHeader(http.StatusNoContent)
					return
				}
				// Filter subscriptions.
				subscriptions = subscriptions.
					FilterByView(articleFilters.GetView()).
					FilterByIDs(articleFilters.GetSubscriptions()...)

				if len(subscriptions) == 0 {
					res.WriteHeader(http.StatusNoContent)
					return
				}

				// Get categories for items.
				counts, err := service.GetTopCategoriesForItems(
					req.Context(),
					query.Bool(
						query.Should(models.BuildItemQueries(user, articleFilters.GetView(), subscriptions)...),
					),
				)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not get item categories.",
						slog.Any("error", err),
					)
					RenderPartial(&PartialTemplate{
						template: templates.CategoryFilters(&models.CategoryFilters{}),
					}).ServeHTTP(res, req)
					return
				}

				// Generate the categories list template.
				RenderPartial(&PartialTemplate{
					template: templates.CategoryFilters(
						&models.CategoryFilters{
							Categories: counts,
							Path:       "/list/articles",
							Filters:    *articleFilters,
						},
					),
				}).ServeHTTP(res, req)
			default:
				res.WriteHeader(http.StatusNotAcceptable)
			}
		}).ServeHTTP
}
