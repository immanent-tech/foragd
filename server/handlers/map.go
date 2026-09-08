// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/pkg/htmx"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/components"
	"github.com/immanent-tech/foragd/web/templates/element"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

type MapArticles struct {
	title    templates.PageTitle
	template templ.Component
}

func (p *MapArticles) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
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
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func HandleMap() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Build request object.
		request := &models.ListRequest{
			Filters: *models.ListFiltersFromCtx(req.Context()),
		}
		if err := request.Validate(); err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("parse query values: %w", err),
			).ServeHTTP(res, req)
			return
		}

		var (
			articles     models.Articles
			subscription *models.Subscription
			err          error
		)

		// Get the subscription details if the list is for a specific subscription.
		var subscriptionID models.SubscriptionID
		if len(request.Filters.GetSubscriptions()) == 1 {
			subscriptionID = request.Filters.GetSubscriptions()[0]
		} else if req.FormValue("subscription_id") != "" {
			subscriptionID = req.FormValue("subscription_id")
		}
		if subscriptionID != "" {
			subscription, err = service.GetSubscription(
				req.Context(),
				subscriptionID,
			)
			if err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("get subscription details: %w", err),
				).ServeHTTP(res, req)
				return
			}
			if user.GetSettings().ShowSubscriptionStats {
				if err := service.UpdateSubscriptionDynamicInfo(
					req.Context(),
					models.Subscriptions{subscription},
				); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Could not generate subscription dynamic info.",
						slog.Any("error", err))
				}
			}
			request.Query = query.Bool(
				// Filter to items with geo data.
				query.Filter(
					query.Exists("geo"),
				),
				// Must match subscription article filters.
				service.ArticleFiltersQueryClause(subscription.GetArticleFilters()),
			)
		} else {
			request.Query = query.Bool(
				// Filter to items with geo data.
				query.Filter(
					query.Exists("geo"),
				),
			)
		}

		// Get articles matching filters.
		var next models.Pagination
		// TODO: Currently hard-coded last 50 articles with geo coords. Decide how to expose as user control.
		request.Filters.Count = 50
		articles, next, err = service.FilterArticles(req.Context(), request)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("filter articles: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// Create response.
		response := &models.ListArticlesResponse{
			Subscription: subscription,
			Articles:     articles,
			Filters:      request.Filters,
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
					"/list/articles",
					response.Filters,
					response.Articles.GetCategoryCounts().GetCategories(),
				),
			}).ServeHTTP(res, req)
		}
	}
}

func HandleMapUpdates() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters := models.ListFiltersFromCtx(req.Context())

		// Don't bother calculating updates if user is not viewing unread items.
		if filters.GetView() != models.ViewUnread {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Retrieve the user object.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		// Retreive subscription details.
		subscriptions, err := service.GetAllSubscriptions(req.Context())
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			slogctx.FromCtx(req.Context()).Error("Failed to get user subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
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
			FilterByView(filters.GetView()).
			FilterByIDs(filters.GetSubscriptions()...)
		if len(subscriptions) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Generate a query to find updates.
		updatesQuery := query.Bool(
			query.Filter(
				query.Exists("geo"),
				// Published/updated within the last 5 minutes.
				query.Bool(
					query.Should(
						query.Since("published", time.Now().UTC().Add(-5*time.Minute)),
						query.Since("updated", time.Now().UTC().Add(-5*time.Minute)),
					),
				),
				// Must match any of the given feed IDs.
				query.Terms("feed_id", subscriptions.GetFeedIDs()),
				// Must match any of the given categories.
				query.Terms("categories.raw", filters.GetCategories()),
				// And should match one feed clause.
				query.Bool(
					query.Filter(service.BuildItemQueries(user, filters.GetView(), subscriptions)...),
				),
			),
		)

		// Count items matching.
		updateCount, err := service.CountItems(req.Context(), updatesQuery)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get updates.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// If updates found, render a notification.
		if updateCount > 0 {
			RenderPartial(&PartialTemplate{template: partials.UpdatesToast(
				element.WithHXMethod(http.MethodGet, "/map"),
				element.WithHXTarget(templates.ContentID.Target()),
				element.WithHXSwap("morph:innerHTML scroll:top transition:true"),
				element.WithHXPushURL(true),
				element.WithHXValues(filters),
			)}).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}
}
