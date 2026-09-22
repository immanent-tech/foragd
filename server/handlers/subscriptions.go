// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// SubscriptionCtx retrieves the subscription matching the URL param and stores it in the context.
func AllSubscriptionsCtx(svc SubscriptionsService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			subscriptions, err := svc.GetAllSubscriptions(req.Context())
			if err != nil && !errors.Is(err, models.ErrNotFound) {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("get all user subscriptions: %w", err),
				).ServeHTTP(res, req)
				return
			}
			ctx := models.SubscriptionsToCtx(req.Context(), subscriptions)
			if err := svc.UpdateSubscriptionDynamicInfo(ctx, subscriptions); err != nil {
				slogctx.Warn(req.Context(), "Could not update subscription dynamic info.",
					slog.Any("error", err))
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SubscriptionCtx retrieves the subscription matching the URL param and stores it in the context.
func SubscriptionCtx(svc SubscriptionsService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			id := chi.URLParam(req, "subscriptionID")
			subscription, err := svc.GetSubscription(req.Context(), id)
			if err != nil {
				HandleInternalError(
					http.StatusUnprocessableEntity,
					fmt.Errorf("fetch subscription %s details: %w", id, err),
				).ServeHTTP(res, req)
				return
			}
			if subscription == nil {
				HandleInternalError(
					http.StatusNotFound,
					fmt.Errorf("fetch subscription %s: not found", id),
				).ServeHTTP(res, req)
				return
			}
			ctx := models.SubscriptionToCtx(req.Context(), subscription)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// ListSubscriptions holds data for generating the subscriptions list page.
type ListSubscriptions struct {
	title    templates.PageTitle
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (p *ListSubscriptions) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of subscriptions, the controls and update the title/dock/sidebar or, when
// paginating, just the list of subscriptions.
func (p *ListSubscriptions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/list/subscriptions":
		res.Header().Set(htmx.HeaderPushURL, req.URL.String())
		templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
		templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
		templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
		templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	case "/subscriptions/paginate":
		templ.Handler(p.template, templ.WithFragments(templates.PaginateFragment)).ServeHTTP(res, req)
	}
}

// HandleListSubscriptions handles displaying a list of subscriptions.
func HandleListSubscriptions(subscriptionSvc SubscriptionsService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		subscriptions := models.SubscriptionsFromCtx(req.Context())
		if subscriptions == nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound),
			).ServeHTTP(res, req)
			return
		}

		// Generate request object.
		request := &models.ListRequest{
			Filters: *ListFiltersFromCtx(req.Context()),
		}
		if err := request.Validate(); err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				fmt.Errorf("validate request: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// If the user has requested to hide grouped subscriptions, filter those out.
		hiddenSubscriptions := make([]models.SubscriptionID, 0)
		if user.GetSettings().HideGrouped {
			for subscription := range slices.Values(subscriptions.FilterByType(models.SubscriptionTypeGroup)) {
				hiddenSubscriptions = append(hiddenSubscriptions, subscription.GroupData.GetGroupedSubscriptionIDs()...)
			}
		}
		subscriptions = subscriptions.ExcludeIDs(hiddenSubscriptions...)

		// Get subscriptions with filters applied.
		var pagination models.Pagination
		subscriptions, pagination = subscriptions.
			FilterByView(request.Filters.GetView()).
			FilterByCategories(request.Filters.GetCategories()...).
			FilterByIDs(request.Filters.GetSubscriptions()...).
			Sort(request.Filters.GetSort()).
			Paginate(&request.Filters)

		// Get latest articles for subscriptions.
		if len(subscriptions) > 0 {
			subscriptionSvc.GetLatestArticles(req.Context(), request.Filters.GetView(), subscriptions)
		}

		// Create response object
		response := &models.ListSubscriptionsResponse{
			Filters:       request.Filters,
			Subscriptions: subscriptions,
		}
		// Update filters in response.
		response.Filters.From = pagination.From
		if request.Filters.UpTo != nil {
			response.Filters.UpTo = nil
		}

		// If the list of articles is from a single subscription, update the page tile to include the subscription
		// name.
		title := templates.PageTitle{
			Summary:     "Subscriptions",
			Description: string(response.Filters.GetView()) + " | " + response.Filters.GetSort().String(),
		}

		// Choose rendering method based on method (get = page, post = partial).
		// ctx := service.ListFiltersToCtx(req.Context(), request.Filters)
		switch req.Method {
		case http.MethodGet:
			RenderInternalPage(&ListSubscriptions{
				title:    title,
				template: templates.ListSubscriptions(response),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			// Render new subscription cards.
			RenderPartial(&ListSubscriptions{
				title:    title,
				template: templates.ListSubscriptions(response),
			}).ServeHTTP(res, req)
			// Render new category filters.
			RenderPartial(&PartialTemplate{
				template: templates.UpdateListCategoryFilters(
					"/list/subscriptions",
					response.Filters,
					response.Subscriptions.GetCategories(),
				),
			}).ServeHTTP(res, req)
			// Update pagination control element.
			if response.Filters.From != nil && len(response.Subscriptions) == response.Filters.GetCount() {
				RenderPartial(&PartialTemplate{
					template: templates.ListPaginationControl(
						"/subscriptions/paginate",
						&response.Filters,
						element.WithHXSwapOOB("true"),
					),
				}).ServeHTTP(res, req)
			}
		}
	}
}

// HandleListSubscriptionsUpdates handles checking for any updates and notifying the user.
func HandleListSubscriptionsUpdates(itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters := ListFiltersFromCtx(req.Context())

		// Don't bother calculating updates if user is not viewing unread items.
		if filters.GetView() != models.ViewUnread {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Get user details.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get user data.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		allSubscriptions := models.SubscriptionsFromCtx(req.Context())
		if allSubscriptions == nil {
			slogctx.FromCtx(req.Context()).Error("Get all subscriptions failed.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Apply all base filtering and sorting.
		subscriptions := allSubscriptions.
			FilterByView(filters.GetView()).
			FilterByCategories(filters.GetCategories()...).
			FilterByIDs(filters.GetSubscriptions()...)
		if len(subscriptions) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		updatesQuery := query.Bool(
			query.WithBoolQueryName("list_subscriptions_updates_"+user.GetID()),
			query.Filter(
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
		updateCount, err := itemSvc.CountItems(req.Context(), updatesQuery)
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
				element.WithHXMethod(http.MethodGet, "/list/subscriptions"),
				element.WithHXTarget(templates.ContentID.Target()),
				element.WithHXSwap("morph:innerHTML scroll:top transition:true"),
				element.WithHXValues(filters),
			)}).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}
}

// HandleMarkSubscription handles marking a subscription as read/unread and updates the UI accordingly.
func HandleMarkSubscription(
	subSvc SubscriptionsService,
	session SessionManager,
	breadcrumbs Breadcrumbs,
	mark models.Mark,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(models.ActionHeader, "mark-subscription")

		// Retrieve the subscription details.
		subscription := models.SubscriptionFromCtx(req.Context())
		if subscription == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no subscription in context")).ServeHTTP(res, req)
			return
		}

		// Mark subscription.
		if err := subSvc.MarkSubscriptions(req.Context(), mark, subscription.GetID()); err != nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("mark subscriptions: %w", err),
			).ServeHTTP(res, req)
			return
		}

		prev, _ := breadcrumbs.Previous(req.Context())

		// Update toggle.
		RenderPartial(&PartialTemplate{
			template: templates.SubscriptionMarkToggle(subscription,
				element.WithHXSwapOOB("true"),
			),
		}).ServeHTTP(res, req)

		// Perform post handling hooks.
		var postMarkHooks = map[string]PostHandlerHook{
			"/list/subscriptions": postMarkSubscriptionList,
			"/list/articles":      postMarkSubscriptionArticles(session),
		}
		if hook, ok := postMarkHooks[prev]; ok {
			if err := hook(res, req); err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("run post mark hook: %w", err),
				).ServeHTTP(res, req)
				return
			}
		}

		res.WriteHeader(http.StatusOK)
	}
}

// postMarkSubscriptionList performs post-mark steps when the subscription was marked from the list subscriptions page.
func postMarkSubscriptionList(res http.ResponseWriter, req *http.Request) error {
	subscription := models.SubscriptionFromCtx(req.Context())
	if subscription == nil {
		return fmt.Errorf("no subscription in context")
	}
	filters := ListFiltersFromCtx(req.Context())
	// If we aren't viewing all subscriptions, remove the subscription card.
	if models.View(filters.GetView()) != models.ViewAll {
		res.Header().Set(htmx.HeaderReswap, "delete transition:true swap:300ms")
		res.Header().Set(htmx.HeaderRetarget, htmx.ID(subscription.GetID()).Target())
		res.Header().Set(htmx.HeaderTrigger, "masonry:update")
		res.Header().Set(models.ActionHeader, "mark-subscription")
	}
	return nil
}

// postMarkSubscriptionArticles performs post-mark steps when the subscription was marked from the list articles page.
func postMarkSubscriptionArticles(session SessionManager) PostHandlerHook {
	return func(res http.ResponseWriter, req *http.Request) error {
		subscription := models.SubscriptionFromCtx(req.Context())
		if subscription == nil {
			return fmt.Errorf("no subscription in context")
		}
		htmx.LocationResponse(
			htmx.WithLocationPath("/list/subscriptions"),
			htmx.WithLocationTarget(templates.ContentID.Target()),
			htmx.WithLocationSwap("morph:innerHTML transition:true"),
			htmx.WithLocationHeaders(map[string]string{
				models.ActionHeader: "mark-subscription",
			}),
			htmx.WithLocationValues(ListFiltersFromSession(req.Context(), session, "/list/subscriptions")),
		).ServeHTTP(res, req)
		return nil
	}
}

// HandleBulkMarkSubscriptions handles bulk marking subscriptions as read/unread.
func HandleBulkMarkSubscriptions(svc SubscriptionsService, mark models.Mark) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(models.ActionHeader, "bulk-mark-subscriptions")

		// Decode request parameters.
		request, err := parseForm[*models.BulkMarkSubscriptionsRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		switch request.Confirmed {
		case false:
			// Show modal to confirm bulk mark articles.
			RenderPartial(&Modal{
				template: templates.BulkMarkSubscriptionsModal(mark,
					element.WithHXSwap("none"),
				)}).ServeHTTP(res, req)
		case true:
			// Determine actions to apply based on which route this handler was called from.
			ctx := req.Context()
			switch {
			case strings.Contains(req.Referer(), "/user/settings"):
				ctx = templates.FragmentKeysToCtx(req.Context(), templates.SubscriptionsTable)
				res.Header().Set(htmx.HeaderRefresh, "true")
			default:
				res.Header().Set(htmx.HeaderRefresh, "true")
			}
			// Mark selected subscriptions.
			if err = svc.MarkSubscriptions(ctx, mark, request.Subscriptions...); err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("mark subscriptions: %w", err),
				).ServeHTTP(res, req.WithContext(ctx))
				return
			}
		}

		res.WriteHeader(http.StatusOK)
	}
}

// HandleFavoriteSubscription handles managing a favorite subscription for a user.
func HandleFavoriteSubscription(svc SubscriptionsService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(models.ActionHeader, "favorite-subscription")

		// Retrieve the subscription details.
		subscription := models.SubscriptionFromCtx(req.Context())
		if subscription == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no subscription in context")).ServeHTTP(res, req)
			return
		}

		// Toggle the subscription state.
		subscription.Favorite = !subscription.IsFavorite()
		// Update subscription
		if err := svc.UpdateSubscriptions(req.Context(), subscription); err != nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("update subscription: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// Update toggle.
		RenderPartial(&PartialTemplate{
			template: templates.SubscriptionFavoriteToggle(subscription,
				element.WithHXSwapOOB("true"),
			),
		}).ServeHTTP(res, req)

		res.WriteHeader(http.StatusOK)
	}
}

// HandleRemoveSubscription handles removing (unsubscribing) from a subscription.
func HandleRemoveSubscription(svc SubscriptionsService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(models.ActionHeader, "remove-subscription")

		// Retrieve the subscription details.
		subscription := models.SubscriptionFromCtx(req.Context())
		if subscription == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no subscription in context")).ServeHTTP(res, req)
			return
		}

		// Decode request parameters.
		request, err := parseForm[*models.ConfirmRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		switch request.Confirmed {
		case false:
			if strings.Contains(req.Referer(), "/list/subscriptions") {
				// On "/list/subscriptions", remove the subscription card.
				RenderPartial(&Modal{
					template: templates.RemoveSubscriptionModal(subscription,
						element.WithHXTarget("#"+subscription.GetID()),
						element.WithHXSwap("delete transition:true"),
					)}).ServeHTTP(res, req)
			} else {
				// On "/list/articles", don't do anything to the page (a redirect will be triggered).
				RenderPartial(&Modal{
					template: templates.RemoveSubscriptionModal(subscription,
						element.WithHXSwap("none"),
					)}).ServeHTTP(res, req)
			}
		case true:
			if err := svc.RemoveSubscriptions(req.Context(), subscription.GetID()); err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("remove subscriptions: %w", err),
				).ServeHTTP(res, req)
				return
			}
			switch {
			case strings.Contains(req.Referer(), "/list/articles"):
				// When the current page is "/list/articles", redirect the user to "/list/subscriptions".
				res.Header().Add(htmx.HeaderRedirect, "/list/subscriptions")
			case strings.Contains(req.Referer(), "/user/settings"):
				// When on the subscriptions settings page, remove the subscription from the table.
				res.Header().Set(htmx.HeaderReswap, "delete transition:true swap:300ms")
				res.Header().Set(htmx.HeaderRetarget, "#"+subscription.GetID())
			}
			// Show success notification.
			RenderPartial(
				&Notification{
					msg: models.NewSuccessMessage("Unsubscribed from "+subscription.GetTitle(), ""),
				},
			).ServeHTTP(res, req)
		}
	}
}

// EditSubscription contains the data for rendering a page for editing a subscription.
type EditSubscription struct {
	title    templates.PageTitle
	template templ.Component
}

func (p *EditSubscription) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

func (p *EditSubscription) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
}

// HandleEditSubscription handles presenting the user with a form for editing a subscription.
func HandleEditSubscription(subSvc SubscriptionsService, breadcrumbs Breadcrumbs) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(models.ActionHeader, "edit-subscription")

		// Retrieve the subscription details.
		subscription := models.SubscriptionFromCtx(req.Context())
		if subscription == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no subscription in context")).ServeHTTP(res, req)
			return
		}

		var template templ.Component
		var pageTitle templates.PageTitle
		ctx := req.Context()
		switch subscription.GetSubscriptionType() {
		case models.SubscriptionTypeFeed:
			// Convert metadata into edit request data.
			request := &models.FeedSubscriptionRequest{
				SubscriptionID: subscription.GetID(),
				Customisation:  subscription.Customisation,
				Settings:       &subscription.Settings,
				ArticleFilters: subscription.FeedData.ArticleFilters,
			}
			// Get top suggestedCategories across items in subscription feed and add as suggested suggestedCategories for the
			// subscription.
			request.SuggestedCategories = subSvc.GetSubscriptionCategorySuggestions(
				req.Context(),
				[]models.FeedID{subscription.FeedData.GetFeedID()},
				subscription.Customisation.Categories,
			)
			// Generate page template.
			template = templates.EditFeedSubscription(request, breadcrumbs)
			pageTitle = templates.PageTitle{
				Summary:     "Edit Subscription",
				Description: subscription.GetTitle(),
			}
		case models.SubscriptionTypeSearch:
			// Editing SearchSubscription.
			request := &models.SearchSubscriptionRequest{
				Customisation: subscription.Customisation,
				Settings:      &subscription.Settings,
				Search:        subscription.SearchData.Search,
			}
			// Get suggested categories from existing subscriptions.
			request.SuggestedCategories = getCategorySuggestions(ctx).Limit(10).GetCategories()

			request.Search.SubscriptionID = new(subscription.GetID())
			// // Get any extra subscription info for subscription filters.
			// if len(request.Search.Subscriptions) > 0 {
			// 	subscriptions, err := service.GetSubscriptionsByID(ctx, request.Search.Subscriptions...)
			// 	if err != nil {
			// 		HandleInternalError(
			// 			http.StatusInternalServerError,
			// 			fmt.Errorf("get subscriptions by ID: %w", err),
			// 		).ServeHTTP(res, req)
			// 		return
			// 	}
			// 	ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			// }
			// Generate page template.
			template = templates.EditSearchSubscription(request, breadcrumbs)
			pageTitle = templates.PageTitle{
				Summary:     "Edit Subscription",
				Description: request.Customisation.GetNickname(),
			}
		case models.SubscriptionTypeGroup:
			// Get all subscriptions.
			allSubscriptions := models.SubscriptionsFromCtx(req.Context())
			if allSubscriptions == nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound),
				).ServeHTTP(res, req)
				return
			}
			// Get the grouped subscriptions.
			groupedSubscriptions := allSubscriptions.FilterByIDs(
				subscription.GroupData.GetGroupedSubscriptionIDs()...)
			if len(groupedSubscriptions) == 0 {
				HandleInternalError(
					http.StatusInternalServerError,
					errors.New("no grouped subscriptions!"),
				).ServeHTTP(res, req)
				return
			}
			// Create the request with details from the group subscription.
			request := &models.GroupSubscriptionRequest{
				Customisation:  subscription.Customisation,
				Settings:       new(subscription.Settings),
				Subscriptions:  make(map[models.SubscriptionID]string),
				SubscriptionID: new(subscription.GetID()),
				ArticleFilters: subscription.GroupData.ArticleFilters,
			}
			// Populate the subscriptions data in the request.
			for subscription := range slices.Values(groupedSubscriptions) {
				request.Subscriptions[subscription.GetID()] = subscription.GetTitle()
			}
			// Get top suggestedCategories across items in subscription feed and add as suggested suggestedCategories
			// for the subscription.
			request.SuggestedCategories = subSvc.GetSubscriptionCategorySuggestions(
				req.Context(),
				groupedSubscriptions.GetFeedIDs(),
				groupedSubscriptions.GetCategories(),
			)
			request.SuggestedSubscriptions = allSubscriptions.
				FilterByType(models.SubscriptionTypeFeed).
				ExcludeIDs(subscription.GroupData.GetGroupedSubscriptionIDs()...)

			// Generate page template.
			template = templates.EditGroupSubscription(request, breadcrumbs)
			pageTitle = templates.PageTitle{
				Summary:     "Edit Subscription",
				Description: request.Customisation.GetNickname(),
			}
		case models.SubscriptionTypeEmail:
			// Editing SearchSubscription.
			request := &models.EditEmailSubscriptionRequest{
				Customisation:  subscription.Customisation,
				Settings:       new(subscription.Settings),
				SubscriptionID: subscription.GetID(),
			}
			// Get suggested categories from existing subscriptions.
			request.SuggestedCategories = getCategorySuggestions(ctx).Limit(10).GetCategories()
			// Create template.
			template = templates.EditEmailSubscription(request, breadcrumbs)
			pageTitle = templates.PageTitle{
				Summary:     "Edit Subscription",
				Description: request.Customisation.GetNickname(),
			}
		}
		// Render the page.
		RenderInternalPage(
			&EditSubscription{
				title:    pageTitle,
				template: template,
			},
		).ServeHTTP(res, req.WithContext(ctx))
	}
}

func getCategorySuggestions(ctx context.Context, ids ...models.SubscriptionID) models.CategoryCounts {
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if allSubscriptions == nil {
		return nil
	}
	subscriptions := allSubscriptions.FilterByIDs(ids...)
	return subscriptions.GetCategoryCounts()
}

// HandleSaveSubscription handles saving the edits made by a user to a subscription.
func HandleSaveSubscription(appCfg AppConfig, cache ImageCache, svc SubscriptionsService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set(models.ActionHeader, "save-subscription")

		// Retrieve the subscription details.
		subscription := models.SubscriptionFromCtx(req.Context())
		if subscription == nil {
			HandleInternalError(http.StatusNotFound, fmt.Errorf("no subscription in context")).ServeHTTP(res, req)
			return
		}

		// Generate the appropriate subscription edit request.
		switch models.SubscriptionType(req.FormValue("subscription_type")) {
		case models.SubscriptionTypeFeed:
			request, err := parseMultipartForm[*models.FeedSubscriptionRequest](req)
			if err != nil {
				HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
				return
			}
			if err := service.EditFeedSubscription(req.Context(), subscription, request); err != nil {
				HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
				return
			}
		case models.SubscriptionTypeSearch:
			request, err := parseMultipartForm[*models.SearchSubscriptionRequest](req)
			if err != nil {
				HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
				return
			}
			if err := service.EditSearchSubscription(req.Context(), subscription, request); err != nil {
				HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
				return
			}
		case models.SubscriptionTypeGroup:
			request, err := parseMultipartForm[*models.GroupSubscriptionRequest](req)
			if err != nil {
				HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
				return
			}
			if err := service.EditGroupSubscription(req.Context(), subscription, request); err != nil {
				HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
				return
			}
		case models.SubscriptionTypeEmail:
			request, err := parseMultipartForm[*models.EditEmailSubscriptionRequest](req)
			if err != nil {
				HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
				return
			}
			if err := service.EditEmailSubscription(req.Context(), subscription, request); err != nil {
				HandleInternalError(http.StatusInternalServerError, err).ServeHTTP(res, req)
				return
			}
		}

		// Process any uploaded thumbnail image.
		thumbnail, err := processThumbnail(appCfg, cache, req, subscription.GetID())
		if err != nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("update subscription: %w", err),
			).ServeHTTP(res, req)
			return
		}
		if thumbnail != "" {
			subscription.Customisation.ImageURL = new(thumbnail)
		}

		// Update the subscription object.
		subscription.UpdatedAt = new(time.Now().UTC())
		if err = svc.UpdateSubscriptions(req.Context(), subscription); err != nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("update subscription: %w", err),
			).ServeHTTP(res, req)
			return
		}
		RenderPartial(
			&Notification{
				msg: models.NewSuccessMessage(
					subscription.GetTitle()+" saved", "",
				),
			},
		).ServeHTTP(res, req)
	}
}

// AddSubscription contains the data for rendering a page for editing a subscription.
type AddSubscription struct {
	title    templates.PageTitle
	template templ.Component
}

func (h *AddSubscription) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

func (h *AddSubscription) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

// HandleAddSubscription handles showing a form for adding a new subscription.
func HandleAddSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}
		switch {
		case user.Metadata.SubscriptionLimit != nil && user.Metadata.SubscriptionLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrSubscriptionLimitExceeded).ServeHTTP(res, req)
			return
		case user.Metadata.NewsletterLimit != nil && user.Metadata.NewsletterLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrEmailNewsletterLimitExceeded).ServeHTTP(res, req)
			return
		}

		// Get any pre-entered URL (i.e., incoming share links/protocol handlers).
		var (
			givenURL *url.URL
			err      error
		)
		if v := req.FormValue("text"); v != "" && req.FormValue("url") == "" {
			// If URL param is empty and text param is not empty, use text.
			givenURL, err = url.Parse(v)
		}
		if v := req.FormValue("url"); v != "" {
			givenURL, err = models.NormalizeFeedURL(v)
		}
		if err != nil {
			HandleInternalError(
				http.StatusUnprocessableEntity,
				err,
				WithUserMessage(
					models.NewWarningMessage("Unable to parse given URL", "Please check your input and try again"),
				),
			).ServeHTTP(res, req)
			return
		}

		request := &models.FeedSubscriptionRequest{
			// Get suggested categories from existing subscriptions.
			SuggestedCategories: getCategorySuggestions(req.Context()).Limit(10).GetCategories(),
		}
		if givenURL != nil {
			request.URL = givenURL.String()
		}
		res.Header().Set(htmx.HeaderPushURL, req.URL.String())
		RenderInternalPage(
			&AddSubscription{
				title: templates.PageTitle{
					Summary:     "Add Subscription",
					Description: "New Feed Subscription",
				},
				template: templates.AddFeedSubscription(request),
			},
		).ServeHTTP(res, req)
	}
}

// HandleAddNewFeedSubscription handles adding a new feed subscription for a user.
func HandleAddNewFeedSubscription(
	subscriptions SubscriptionsService,
	users UserService,
	feeds FeedService,
	httpClient *resty.Client,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := parseMultipartForm[*models.AddFeedSubscriptionRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		if user := models.UserFromCtx(req.Context()); user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		slogctx.FromCtx(req.Context()).Debug("Processing add feed subscription.",
			slog.String("feed_url", request.URL),
		)

		// Fetch the feed details from the database.
		feed, err := feeds.GetFeed(req.Context(), request.FeedID)
		if err != nil || feed == nil {
			// Fetch the feed details from the URL.
			slogctx.FromCtx(req.Context()).Debug("Fetching new feed details.",
				slog.String("feed_url", request.URL),
			)
			feed, err = service.FetchFeed(
				req.Context(),
				httpClient,
				request.URL,
				service.FetchWithFeedID(request.FeedID),
			)
			if err != nil {
				HandleInternalError(
					http.StatusUnprocessableEntity,
					fmt.Errorf("new feed from URL: %w", err),
				).ServeHTTP(res, req)
				return
			}
			// Add the feed to the database.
			if err := feeds.AddFeed(req.Context(), feed); err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("create feed: %w", err),
				).ServeHTTP(res, req)
				return
			}
			slogctx.Info(req.Context(), "Added new feed.",
				slog.String("feed_url", request.URL),
				slog.String("feed_id", feed.GetID()),
				slog.String("feed_title", feed.GetTitle()),
			)
		}

		// Create a new subscription.
		subscription, err := service.NewFeedSubscription(req.Context(), feed, nil)
		if err != nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("create subscription: %w", err),
			).ServeHTTP(res, req)
			return
		}

		// Add subscription to user.
		if err := addSubscriptions(req.Context(), users, subscriptions, subscription); err != nil {
			HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("add subscription: %w", err),
			).ServeHTTP(res, req)
			return
		}
		slogctx.FromCtx(req.Context()).Info("Added user subscription.",
			slog.String("feed_id", subscription.GetFeedID()),
			slog.String("subscription_id", subscription.GetID()),
			slog.String("subscription_title", subscription.GetTitle()),
		)

		RenderPartial(&Notification{
			msg: models.NewSuccessMessage(
				"Subscription Created!",
				subscription.GetTitle(),
			),
		}).ServeHTTP(res, req)
	}
}

func HandleSuggestFeeds(feeds FeedService, httpClient *resty.Client) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Get suggestion text.
		text := validation.SanitizeString(req.FormValue("suggestion_text"))
		source := validation.SanitizeString(req.FormValue("suggestion_source"))
		// Ignore empty text string.
		if text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		switch source {
		case "youtube":
			results, err := feeds.SuggestYoutubeFeeds(req.Context(), text)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable generate youtube suggestions.",
					slog.Any("error", err),
				)
				RenderPartial(&PartialTemplate{
					template: templates.ShowNoSuggestions(text),
				}).ServeHTTP(res, req)
				return
			}
			RenderPartial(&PartialTemplate{
				template: templates.ShowFeedSuggestions(results),
			}).ServeHTTP(res, req)
			return
		case "gnews":
			results, err := feeds.SuggestGoogleNewsFeeds(req.Context(), httpClient, text)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable generate google news suggestions.",
					slog.Any("error", err),
				)
				RenderPartial(&PartialTemplate{
					template: templates.ShowNoSuggestions(text),
				}).ServeHTTP(res, req)
				return
			}
			RenderPartial(&PartialTemplate{
				template: templates.ShowFeedSuggestions(results),
			}).ServeHTTP(res, req)
			return
		case "web":
			fallthrough
		default:
			results, err := feeds.SuggestFeeds(
				req.Context(),
				httpClient,
				&models.SuggestFeedsRequest{Text: text, Count: 10},
			)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable generate feed suggestions.",
					slog.Any("error", err),
				)
				RenderPartial(&PartialTemplate{
					template: templates.ShowNoSuggestions(text),
				}).ServeHTTP(res, req)
				return
			}
			RenderPartial(&PartialTemplate{
				template: templates.ShowFeedSuggestions(results),
			}).ServeHTTP(res, req)
			return
		}
	}
}

// HandleAddSearchSubscription handles adding a new search subscription.
func HandleAddSearchSubscription(
	subSvc SubscriptionsService,
	userSvc UserService,
	breadcrumbs Breadcrumbs,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}
		switch {
		case user.Metadata.SubscriptionLimit != nil && user.Metadata.SubscriptionLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrSubscriptionLimitExceeded).ServeHTTP(res, req)
			return
		case user.Metadata.NewsletterLimit != nil && user.Metadata.NewsletterLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrEmailNewsletterLimitExceeded).ServeHTTP(res, req)
			return
		}

		switch req.Method {
		case http.MethodGet:
			// Get the details.
			request, err := parseForm[*models.SearchRequest](req)
			if err != nil {
				HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			}
			// Render form.
			RenderInternalPage(
				&AddSubscription{
					title: templates.PageTitle{
						Summary:     "Add Subscription",
						Description: "New Search Subscription",
					},
					template: templates.AddSearchSubscription(
						&models.SearchSubscriptionRequest{
							Search:        *request,
							Customisation: &models.SubscriptionCustomisation{},
							SuggestedCategories: getCategorySuggestions(
								req.Context(),
							).Limit(10).
								GetCategories(),
						},
						breadcrumbs,
					),
				},
			).ServeHTTP(res, req.WithContext(req.Context()))
		case http.MethodPost:
			request, err := parseMultipartForm[*models.SearchSubscriptionRequest](req)
			if err != nil {
				HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			}
			subscription, err := service.NewSearchSubscription(req.Context(), request)
			if err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("create search subscription: %w", err),
				).ServeHTTP(res, req)
			}
			if err := subscription.Validate(); err != nil {
				HandleInternalError(
					http.StatusUnprocessableEntity,
					fmt.Errorf("create search subscription: %w", err),
				).ServeHTTP(res, req)
			}
			if err := addSubscriptions(req.Context(), userSvc, subSvc, subscription); err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("add search subscription: %w", err),
				).ServeHTTP(res, req)
			}

			RenderPartial(
				&Notification{
					msg: models.NewSuccessMessage("Search Subscription Created!", ""),
				},
			).ServeHTTP(res, req)
		}
	}
}

func HandleSuggestSubscriptionForSearch(svc SubscriptionsService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := forms.DecodeForm[*models.GetSubscriptionsSuggestionRequest](req)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not suggest subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		subscriptions, err := svc.GetSubscriptionSuggestions(
			req.Context(),
			request.Text,
			10,
			request.IgnoredSubscriptions,
		)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not suggest subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		RenderPartial(&PartialTemplate{
			template: partials.SubscriptionSuggestions(subscriptions),
		}).ServeHTTP(res, req)
	}
}

func HandleAddSubscriptionToSearch() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := forms.DecodeForm[*models.AddSubscriptionToSearchRequest](req)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not suggest subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		for key, value := range request.SuggestedSubscriptions {
			if value == request.SelectedSubscription {
				RenderPartial(&PartialTemplate{
					template: templates.AddSearchSubscriptionFilter(&models.AddSubscriptionSearchFilterRequest{
						InputName:        "search.subscriptions",
						SubscriptionID:   key,
						SubscriptionName: value,
					}),
				}).ServeHTTP(res, req)
				return
			}
		}
		res.WriteHeader(http.StatusNoContent)
	}
}

// HandleAddGroupSubscription handles adding a new group subscription.
func HandleAddGroupSubscription(
	subSvc SubscriptionsService,
	userSvc UserService,
	breadcrumbs Breadcrumbs,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}
		switch {
		case user.Metadata.SubscriptionLimit != nil && user.Metadata.SubscriptionLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrSubscriptionLimitExceeded).ServeHTTP(res, req)
			return
		case user.Metadata.NewsletterLimit != nil && user.Metadata.NewsletterLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrEmailNewsletterLimitExceeded).ServeHTTP(res, req)
			return
		}

		switch req.Method {
		case http.MethodGet:
			// Get suggested categories from existing subscriptions.
			suggestedCategories := getCategorySuggestions(req.Context()).Limit(10).GetCategories()
			// Get suggested suggested subscriptions.
			allSubscriptions := models.SubscriptionsFromCtx(req.Context())
			if allSubscriptions == nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound),
				).ServeHTTP(res, req)
				return
			}
			suggestedSubscriptions := allSubscriptions.FilterByType(models.SubscriptionTypeFeed)

			RenderInternalPage(
				&AddSubscription{
					title: templates.PageTitle{
						Summary:     "Add Subscription",
						Description: "New Group Subscription",
					},
					template: templates.AddGroupSubscription(
						models.NewGroupSubscriptionRequest(suggestedSubscriptions, suggestedCategories),
						breadcrumbs,
					),
				},
			).ServeHTTP(res, req)
		case http.MethodPost:
			// Decode request.
			request, err := parseMultipartForm[*models.GroupSubscriptionRequest](req)
			if err != nil {
				HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
				return
			}

			// Generate subscription metadata from request.
			subscription, err := service.NewGroupSubscription(req.Context(), request)
			if err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("new group subscription: %w", err),
				).ServeHTTP(res, req)
				return
			}
			// Validate subscription.
			if err = subscription.Validate(); err != nil {
				HandleInternalError(
					http.StatusUnprocessableEntity,
					fmt.Errorf("validate group subscription: %w", err),
				).ServeHTTP(res, req)
				return
			}
			// Add subscriptions
			if err := addSubscriptions(req.Context(), userSvc, subSvc, subscription); err != nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("add subscriptions: %w", err),
				).ServeHTTP(res, req)
				return
			}
			// Render notification.
			RenderPartial(
				&Notification{
					msg: models.NewSuccessMessage("Group Subscription Created!", ""),
				},
			).ServeHTTP(res, req)
		}
	}
}

func HandleAddSubscriptionToGroup() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Parse add subscription to group request.
		request, err := parseForm[*models.AddSubscriptionToGroupRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		// Ignore request to add subscription that is already in the group.
		if slices.Contains(slices.Collect(maps.Values(request.ExistingSubscriptions)), request.SuggestionText) {
			HandleInternalError(http.StatusConflict, errors.New("subscription already in group")).ServeHTTP(res, req)
			return
		}
		for subscriptionID, subscriptionName := range request.Suggestions {
			if subscriptionName == request.SuggestionText {
				RenderPartial(&PartialTemplate{
					template: templates.AddSubscriptionToGroup(
						subscriptionID,
						subscriptionName,
					),
				}).ServeHTTP(res, req)
				return
			}
		}
		res.WriteHeader(http.StatusNoContent)
	}
}

// ImportSubscriptions contains the data for rendering a page for importing subscriptions.
type ImportSubscriptions struct {
	title    templates.PageTitle
	template templ.Component
}

func (h *ImportSubscriptions) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

func (h *ImportSubscriptions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
}

type ImportSubscriptionsResults struct {
	template templ.Component
}

func (h *ImportSubscriptionsResults) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(h.template).ServeHTTP(res, req)
}

// HandleImportSubscriptions handles assisting the user with importing subscriptions from an external source.
func HandleImportSubscriptions(
	feeds FeedService,
	users UserService,
	subscriptions SubscriptionsService,
	httpClient *resty.Client,
) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}
		switch {
		case user.Metadata.SubscriptionLimit != nil && user.Metadata.SubscriptionLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrSubscriptionLimitExceeded).ServeHTTP(res, req)
			return
		case user.Metadata.NewsletterLimit != nil && user.Metadata.NewsletterLimit.Exceeded:
			HandleInternalError(http.StatusForbidden, models.ErrEmailNewsletterLimitExceeded).ServeHTTP(res, req)
			return
		}

		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			RenderInternalPage(&ImportSubscriptions{
				title: templates.PageTitle{
					Summary:     "Import",
					Description: "Choose source to import subscriptions",
				},
				template: templates.ImportSubscriptions(),
			}).ServeHTTP(res, req)
		// POST: process import.
		case http.MethodPost:
			// Extract OPML file.
			opmlData, err := decodeMultipartFile(req, "source")
			if err != nil {
				HandleInternalError(
					http.StatusUnprocessableEntity,
					fmt.Errorf("decode opml: %w", err),
				).ServeHTTP(res, req)
				return
			}
			opmlFile := &models.OPMLFile{FileUpload: opmlData}
			// Generate subscription requests from OPML file contents.
			requests, err := opmlFile.GenerateRequests()
			if err != nil {
				HandleInternalError(
					http.StatusUnprocessableEntity,
					fmt.Errorf("generate subscription requests: %w", err),
				).ServeHTTP(res, req)
				return
			}
			// Get user's existing subscriptions.
			currentSubscriptions := models.SubscriptionsFromCtx(req.Context())
			if currentSubscriptions == nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound),
				).ServeHTTP(res, req)
				return
			}
			// Check adding new subscription will not cause user to exceed subscriptions limit.
			if len(
				requests,
			)+len(
				currentSubscriptions,
			)-len(
				currentSubscriptions.FilterByType(models.SubscriptionTypeEmail),
			) > models.MaxSubscriptions {
				HandleInternalError(http.StatusForbidden, models.ErrSubscriptionLimitExceeded).ServeHTTP(res, req)
				return
			}

			// Perform bulk import.
			results := bulkImportFeeds(req.Context(), feeds, users, subscriptions, httpClient, requests...)

			// Display all results.
			RenderPartial(&ImportSubscriptionsResults{
				template: templates.ImportSubscriptionsResults(results),
			}).ServeHTTP(res, req)
			// Display notification.
			RenderPartial(&Notification{
				msg: models.NewSuccessMessage(
					"OPML import complete.",
					"Please consult the results and check for any issues.",
				),
			}).ServeHTTP(res, req)
		}
	}
}

// ExportSubscriptions contains the data for rendering a page for exporting subscriptions.
type ExportSubscriptions struct {
	title    templates.PageTitle
	template templ.Component
}

func (h *ExportSubscriptions) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

func (h *ExportSubscriptions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
}

// HandleExportSubscriptions handles configuring and performing an export of user subscriptions.
func HandleExportSubscriptions(feedSvc FeedService, breadcrumbs Breadcrumbs) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Get the user details.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			RenderInternalPage(
				&ExportSubscriptions{
					title: templates.PageTitle{
						Summary:     "Export",
						Description: "Export your subscriptions as OPML",
					},
					template: templates.ExportSubscriptions(breadcrumbs),
				},
			).ServeHTTP(res, req)
		case http.MethodPost:
			// Get all user subscriptions.
			subscriptions := models.SubscriptionsFromCtx(req.Context())
			if subscriptions == nil {
				HandleInternalError(
					http.StatusInternalServerError,
					fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound),
				).ServeHTTP(res, req)
				return
			}
			// Generate opml file.
			opmlFile, err := feedSvc.GenerateOPML(
				req.Context(),
				subscriptions.FilterByType(models.SubscriptionTypeFeed).GetFeedIDs()...)
			if err != nil {
				HandleInternalError(
					http.StatusUnprocessableEntity,
					fmt.Errorf("generate opml: %w", err),
				).ServeHTTP(res, req)
				return
			}

			// Serve the opml content via http.ServeContent.
			res.Header().Set("Content-Type", "text/x-opml+xml; charset=utf-8")
			filename := "Foragd-Export.opml"
			res.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeContent(res, req, filename, time.Now(), bytes.NewReader(opmlFile))
		}
	}
}

// HandleSubscriptionCategories handles adding and removing categories from a subscription, either when editing or
// adding.
func HandleSubscriptionCategories() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		request, err := parseForm[*models.AddCategoryToSubscriptionRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		categories := strings.Split(request.Category, ",")
		if len(categories) == 0 {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		for category := range slices.Values(categories) {
			if slices.Contains(request.ExistingCategories, request.Category) {
				// Ignore existing categories.
				continue
			}
			RenderPartial(&PartialTemplate{template: templates.AddCategory(category)}).ServeHTTP(res, req)
		}
	}
}

func processThumbnail(appCfg AppConfig, cache ImageCache, req *http.Request, objectID string) (string, error) {
	const maxThumbnailSize = 1000000 // Max thumbnail size is 1 MB.

	// Get any uploaded image.
	image, err := decodeMultipartFile(req, "thumbnail")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		return "", fmt.Errorf("parse thumbnail data: %w", err)
	}
	if image.GetSize() > maxThumbnailSize {
		return "", fmt.Errorf("parse thumbnail data: %w", models.ErrFileTooLarge)
	}

	// If the user uploaded a new avatar, process it.
	if image != nil {
		// Generate a unique ID for the avatar image in the cache using the user ID.
		imageFileID := strconv.FormatUint(xxh3.Hash([]byte(objectID+"thumbnail")), 10)
		// Read the uploaded data and store in the cache.
		imageData, err := io.ReadAll(image.Data)
		if err != nil {
			return "", fmt.Errorf("read thumbnail data: %w", err)
		}
		if err := cache.SaveThumbnail(req.Context(), imageFileID, imageData); err != nil {
			return "", fmt.Errorf("save thumbnail: %w", err)
		}
		// Construct a new full URL to the uploaded avatar on the local server.
		return appCfg.GetBaseURL().JoinPath("/img/subscription/" + imageFileID).String(), nil
	}

	return "", nil
}

// bulkImportFeeds handles processing any number of NewFeedSubscriptionRequest requests.
func bulkImportFeeds(
	ctx context.Context,
	feeds FeedService,
	users UserService,
	subscriptions SubscriptionsService,
	httpClient *resty.Client,
	requests ...models.FeedSubscriptionRequest,
) []models.FeedSubscriptionResult {
	// Process requests.
	resultsCh := make(chan models.FeedSubscriptionResult)
	var wg sync.WaitGroup

	for request := range slices.Values(requests) {
		wg.Go(func() {
			// Find an existing or create a new feed from the requested URL.
			feed, isNew, err := feeds.FindOrCreateFeed(ctx, httpClient, request.URL)
			if err != nil {
				result := models.FeedSubscriptionResult{
					Request: &request,
				}
				if apiErr, ok := errors.AsType[*models.APIError](err); ok {
					result.Error = apiErr
				} else {
					result.Error = models.NewAPIError(
						http.StatusUnprocessableEntity,
						fmt.Errorf("create subscription: %w", err),
						models.WithUserErrorSummary("Unable to process feed URL"),
						models.WithUserErrorDescription(request.URL),
					)
				}
				resultsCh <- result
				return
			}
			if isNew {
				// Add the feed if it is new.
				if err := feeds.AddFeed(ctx, feed); err != nil {
					resultsCh <- models.FeedSubscriptionResult{
						Request: &request,
						Error: &models.APIError{
							InternalError: fmt.Errorf("create subscription: %w", err),
							StatusCode:    http.StatusInternalServerError,
							UserMessage: models.NewErrorMessage(
								"Unable to add feed subscription",
								fmt.Sprintf("Could not create a feed for %s (%s)", feed.GetTitle(), request.URL),
							),
						},
					}
					return
				}
			}

			allSubscriptions := models.SubscriptionsFromCtx(ctx)
			existingSubscriptions := allSubscriptions.FilterByFeedIDs(feed.GetID())
			if existingSubscriptions != nil {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: errors.New("create subscription: already subscribed"),
						StatusCode:    http.StatusConflict,
						UserMessage: models.NewWarningMessage(
							"Already subscribed to feed",
							fmt.Sprintf("%s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}

			// Create feed newSubscription.
			newSubscription, err := service.NewFeedSubscription(ctx, feed, nil)
			if err != nil {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: fmt.Errorf("create subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to add subscription",
							fmt.Sprintf("Could create subscription data for feed %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			if err := addSubscriptions(ctx, users, subscriptions, newSubscription); err != nil {
				resultsCh <- models.FeedSubscriptionResult{
					Request: &request,
					Error: &models.APIError{
						InternalError: fmt.Errorf("add subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to add subscription",
							fmt.Sprintf("Could subscribe to feed %s (%s)", feed.GetTitle(), request.URL),
						),
					},
				}
				return
			}
			resultsCh <- models.FeedSubscriptionResult{
				Request:      &request,
				Subscription: newSubscription,
			}
		})
	}
	// Wait for all request processing to complete.
	go func() {
		defer close(resultsCh)
		wg.Wait()
	}()
	results := make([]models.FeedSubscriptionResult, 0, len(requests))
	// Gather results.
	for result := range resultsCh {
		results = append(results, result)
	}

	return results
}

// AddSubscriptions adds the given subscriptions to a user.
func addSubscriptions(
	ctx context.Context,
	users UserService,
	subscriptions SubscriptionsService,
	newSubscriptions ...*models.Subscription,
) error {
	user := models.UserFromCtx(ctx)
	if user == nil {
		return fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound)
	}
	if err := subscriptions.UpdateSubscriptions(ctx, newSubscriptions...); err != nil {
		return fmt.Errorf("update subscriptions: %w", err)
	}
	// Disable onboarding once a subscription has been added.
	if settings := user.GetSettings(); settings.ShowOnboarding {
		settings.ShowOnboarding = false
		// Update the user object.
		if err := users.UpdateUser(ctx, user, map[string]any{
			"settings": settings,
		}); err != nil {
			return fmt.Errorf("update user: %w", err)
		}
	}
	return nil
}
