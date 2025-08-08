// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/davecgh/go-spew/spew"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/validation"
	"github.com/joshuar/go-feed-me/web/templates/pages"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/views"
)

// GetSubscriptions handles showing a filtered collection of subscriptions as cards.
func (a *API) GetSubscriptions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		filters, valid, err := forms.DecodeForm[*models.SubscriptionFilters](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		chain = chain.Append(SavePageState(filters))
		var template templ.Component
		// Get subscriptions matching filters.
		subscriptions, pagination, err := a.filterSubscriptions(req.Context(), filters)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Generate page template.
		template = views.NewSubscriptionsPage(subscriptions, filters, pagination).Template(req)

		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)

		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// GetSubscriptionArticles shows the articles for a subscription.
func (a *API) GetSubscriptionArticles() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Get the subscription ID.
		id := chi.URLParam(req, models.RouteParamSubscription)
		valid, err := validation.ValidateVariable(id, "required,startswith=sub_")
		if !valid || err != nil {
			RenderResponse(RespInvalidInput(err)).ServeHTTP(res, req)
			return
		}
		// Get the filters.
		filters, valid, err := forms.DecodeForm[*models.ArticleFilters](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		filters.Subscriptions = append(filters.Subscriptions, id)
		// // Save the filters to the session.
		// chain = chain.Append(SavePageState(filters))
		// Get articles matching filters.
		articles, pagination, err := a.filterArticles(req.Context(), filters)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Generate articles page.
		resp := models.NewResponse(
			models.WithResponseTemplate(views.NewArticlesPage(articles, filters, pagination).Template(req)),
		)

		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// MarkSubscription handles marking a subscription as read or unread.
func (a *API) MarkSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		// Construct the request from parameters.
		request := &models.MarkSubscriptionsRequest{
			Mark:          models.Mark(chi.URLParam(req, "mark")),
			Subscriptions: []models.SubscriptionID{chi.URLParam(req, "subscription")},
		}
		view := models.View(req.FormValue("view"))
		// Mark subscription.
		err := a.markSubscriptions(req.Context(), request)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		var resp *models.Response
		// If the view is "all" send back the updated subscription card.
		if view == models.ViewAll {
			s, err := a.getSubscriptions(req.Context(), request.Subscriptions...)
			if err != nil || len(s) == 0 || len(s) > 1 {
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
			card := partials.NewSubscriptionContent(s[0])
			resp = models.NewResponse(
				models.WithResponseTemplate(card.Card()),
			)
		}

		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// MarkAllSubscriptions handles marking all subscriptions as read or unread.
func (a *API) MarkAllSubscriptions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		// Construct the request from parameters.
		request := &models.MarkSubscriptionsRequest{
			Mark: models.Mark(chi.URLParam(req, "mark")),
		}
		subscriptions := strings.Split(req.FormValue("subscriptions"), ",")
		if len(subscriptions) == 0 {
			request.Subscriptions = user.GetSubscriptionMetadata().GetIDs()
		} else {
			request.Subscriptions = subscriptions
		}
		view := models.View(req.FormValue("view"))
		// Mark subscriptions.
		err := a.markSubscriptions(req.Context(), request)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Redirect depending on the current view.
		switch view {
		case models.ViewRead, models.ViewUnread:
			chain = chain.Append(SetupRedirect("/home"))
		case models.ViewAll:
			chain = chain.Append(SetupRedirect("/subscriptions"))
		}
		chain.Then(RenderResponse(nil)).ServeHTTP(res, req)
	}
}

// EditSubscription handles fetching and presenting the customisation data for a subscription, for the user to edit.
func (a *API) EditSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		id := chi.URLParam(req, "subscription")
		// Retrieve user object.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		metadata := user.GetSubscriptionMetadata().GetByID(id)
		edit := &models.SubscriptionEdit{
			SubscriptionID: id,
			Title:          metadata.Customisation.Title,
			Categories:     metadata.Customisation.Categories,
		}
		// Get top categories across items in subscription feed.
		var topItemCategories []models.Category
		categories, resp := a.getItemTopCategories(req.Context(), metadata.GetFeedID())
		if resp == nil {
			topItemCategories = categories
		}
		subscriptions, err := a.getSubscriptions(req.Context(), id)
		if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
			RenderResponse(RespBackendError(err)).ServeHTTP(res, req)
			return
		}
		resp = models.NewResponse(
			models.WithResponseTemplate(pages.NewEditSubscriptionPage(edit, subscriptions[0], topItemCategories).Template(req)),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// EditSubscriptionCategories handles adding and removing categories from a subscription.
func (a *API) EditSubscriptionCategories() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
		)
		switch req.Method {
		case http.MethodPost:
			// Add a category.
			currentCategories, _, _ := forms.DecodeForm[*partials.AddSubscriptionCategories](req)
			category := req.FormValue("category")
			if category == "" || (currentCategories != nil && slices.Contains(currentCategories.Categories, category)) {
				chain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
					res.WriteHeader(http.StatusNoContent)
				}).ServeHTTP(res, req)
			} else {
				resp := models.NewResponse(
					models.WithResponseTemplate(partials.AddCategory(category)),
				)
				chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
			}
		case http.MethodDelete:
			// Remove a category.
			chain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
				res.WriteHeader(http.StatusOK)
			}).ServeHTTP(res, req)
		default:
			// Unsupported, do nothing.
			chain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
				res.WriteHeader(http.StatusNoContent)
			}).ServeHTTP(res, req)
		}
	}
}

// SaveSubscription handles saving edits made to a subscription by the user.
func (a *API) SaveSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
			// TriggerStateUpdates,
		)
		// Retrieve user object.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		edits, valid, err := forms.DecodeForm[*models.SubscriptionEdit](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusUnprocessableEntity),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.AlertWarn(
					&models.UserMessage{
						Summary: "Invalid data.",
						Details: "There is invalid or missing values.",
					},
				))))).ServeHTTP(res, req)
			return
		}
		metadata := user.GetSubscriptionMetadata().GetByID(edits.SubscriptionID)
		metadata.Customisation.Title = edits.Title
		metadata.Customisation.Categories = edits.Categories
		err = user.UpdateSubscription(metadata)
		if err != nil {
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusInternalServerError),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.AlertError(
					&models.UserMessage{
						Summary: "Backend error.",
						Details: "The backend had problems trying to save the subscription edits, please try again.",
					},
				))))).ServeHTTP(res, req)
			return
		}
		// Update the user.
		err = a.updateUser(req.Context(), map[string]any{
			"subscriptions": user.GetSubscriptionMetadata(),
		})
		if err != nil {
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseStatusCode(http.StatusInternalServerError),
				models.WithResponseError(err),
				models.WithResponseTemplate(partials.AlertError(
					&models.UserMessage{
						Summary: "Backend error.",
						Details: "The backend had problems trying to save the subscription edits, please try again.",
					},
				))))).ServeHTTP(res, req)
			return
		}
		// subscriptions, err := a.getSubscriptions(req.Context(), edits.SubscriptionID)
		// if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
		// 	chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
		// 	return
		// }
		// // Display the updated subscription.
		// var template templ.Component
		// // currentURL, found := htmx.GetCurrentURL(req)
		// if !found {
		// 	chain.Then(RenderResponse(RespBackendError(nil))).ServeHTTP(res, req)
		// 	return
		// }
		// switch {
		// // case strings.HasSuffix(currentURL, "/user/settings"):
		// // 	template = partials.NewSubscriptionContent(subscriptions[0]).ShowAsSetting()
		// default:
		// 	template = partials.NewSubscriptionContent(subscriptions[0]).Card()
		// }
		// Display a notification acknowledging save (OOB swap).
		chain.Then(RenderResponse(models.NewResponse(
			models.WithResponseTemplate(partials.AlertSuccess(
				&models.UserMessage{
					Summary: "Subscription edits saved!",
				},
			))))).ServeHTTP(res, req)
	}
}

// RemoveSubscription handles presenting the user with confirmation and then actioning a subscription removal
// (unsubscribe) request.
func (a *API) RemoveSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up the handler chain.
		chain := alice.New(
			RouteLogger,
		)
		switch req.Method {
		case http.MethodGet:
			// Show a modal to confirm unsubscribe request.
			id := chi.URLParam(req, "subscription")
			subscriptions, err := a.getSubscriptions(req.Context(), id)
			if err != nil || len(subscriptions) == 0 || len(subscriptions) > 1 {
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
			resp := models.NewResponse(
				models.WithResponseTemplate(partials.NewSubscriptionContent(subscriptions[0]).UnsubscribeModal()),
			)
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
		case http.MethodPost:
			// Perform unsubscribe action.
			id := chi.URLParam(req, "subscription")
			// Retrieve user object.
			user, found := models.UserFromCtx(req.Context())
			if !found {
				chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
				return
			}
			// Remove metadata for given subscriptions from user.
			user.RemoveSubscriptions(id)
			// Update the user.
			err := a.updateUser(req.Context(), map[string]any{
				"subscriptions": user.GetSubscriptionMetadata(),
			})
			if err != nil {
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
			// Show success notification.
			msg := &models.UserMessage{
				Summary: "Unsubscribed.",
				Status:  models.UserMessageStatusSuccess,
			}
			resp := models.NewResponse(
				models.WithResponseTemplate(partials.ShowNotification(msg)),
			)
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
		}
	}
}

// AddSubscription handles adding a new subscription requested by the user.
func (a *API) AddSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		switch req.Method {
		case http.MethodGet:
			resp := models.NewResponse(
				models.WithResponseTemplate(pages.NewAddSubscriptionPage().Template(req)),
			)
			chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
			if err != nil || !valid {
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusUnprocessableEntity),
					models.WithResponseError(err),
					models.WithResponseTemplate(partials.AlertWarn(
						&models.UserMessage{
							Summary: "Invalid data.",
							Details: "There is invalid or missing values.",
						},
					))))).ServeHTTP(res, req)
				return
			}
			requests := addSubscriptionRequests{
				request: &models.Feed{},
			}
			// Match the request to either and existing or new feed.
			result, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a)
			if err != nil {
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(partials.AlertError(
						&models.UserMessage{
							Summary: "Backend error.",
							Details: "The backend had problems trying to add the subscription, please try again.",
						},
					))))).ServeHTTP(res, req)
				return
			}
			// If results returned from matching is non-nil, something went wrong.
			if result[request] != nil {
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusUnprocessableEntity),
					models.WithResponseTemplate(partials.AlertError(result[request].Message)),
				))).ServeHTTP(res, req)
				return
			}
			// Create the new subscription.
			createResult, err := requests.createNewSubscriptions(req.Context(), a)
			if err != nil {
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(partials.AlertError(
						&models.UserMessage{
							Summary: "Backend error.",
							Details: "The backend had problems trying to add the subscription, please try again.",
						},
					))))).ServeHTTP(res, req)
			} else {
				result = createResult
			}
			chain.Then(RenderResponse(models.NewResponse(
				models.WithResponseTemplate(partials.AlertSuccess(
					&models.UserMessage{
						Summary: fmt.Sprintf("Subscription %q created!", result[request].Subscription.GetTitle()),
						Details: "New subscription: " + result[request].Subscription.String(),
					},
				))))).ServeHTTP(res, req)
		}
	}
}

// ImportSubscriptions handles assisting the user with importing subscriptions from an external source.
func (a *API) ImportSubscriptions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		var resp *models.Response
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			resp = models.NewResponse(
				models.WithResponseTemplate(pages.NewImportPage().Template(req)),
			)
		// POST: process import.
		case http.MethodPost:
			requests := make(addSubscriptionRequests)
			opmlFile := &models.OPMLFile{}
			opmlFile, valid, err := forms.DecodeMultipartFile(req, "source", opmlFile)
			if err != nil || !valid {
				htmxResp := htmxRespFromCtx(req.Context())
				htmxResp = htmxResp.Retarget("#notifications")
				ctx := htmxRespToCtx(req.Context(), htmxResp)
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusBadRequest),
					models.WithResponseError(err),
					models.WithResponseTemplate(partials.AlertWarn(
						&models.UserMessage{
							Summary: "Failed to read OPML file.",
							Details: "Could not parse the OPML file, please check and try again.",
						},
					))))).ServeHTTP(res, req.WithContext(ctx))
				return
			}
			spew.Dump(opmlFile, valid, err)
			r, err := opmlFile.GenerateRequests()
			if err != nil {
				htmxResp := htmxRespFromCtx(req.Context())
				htmxResp = htmxResp.Retarget("#notifications")
				ctx := htmxRespToCtx(req.Context(), htmxResp)
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusUnprocessableEntity),
					models.WithResponseError(err),
					models.WithResponseTemplate(partials.AlertWarn(
						&models.UserMessage{
							Summary: "Failed to extract subscriptions from OPML file.",
							Details: "Could not extract subscriptions from the OPML file, please check the contents and try again.",
						},
					))))).ServeHTTP(res, req.WithContext(ctx))
				return
			}
			for newRequest := range slices.Values(r) {
				requests[newRequest] = &models.Feed{}
			}
			matchResults, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a)
			if err != nil {
				htmxResp := htmxRespFromCtx(req.Context())
				htmxResp = htmxResp.Retarget("#notifications")
				ctx := htmxRespToCtx(req.Context(), htmxResp)
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(partials.AlertError(
						&models.UserMessage{
							Summary: "Backend error.",
							Details: "The backend had problems trying to add the subscription, please try again.",
						},
					))))).ServeHTTP(res, req.WithContext(ctx))
				return
			}
			createResults, err := requests.createNewSubscriptions(req.Context(), a)
			if err != nil {
				htmxResp := htmxRespFromCtx(req.Context())
				htmxResp = htmxResp.Retarget("#notifications")
				ctx := htmxRespToCtx(req.Context(), htmxResp)
				chain.Then(RenderResponse(models.NewResponse(
					models.WithResponseStatusCode(http.StatusInternalServerError),
					models.WithResponseError(err),
					models.WithResponseTemplate(partials.AlertError(
						&models.UserMessage{
							Summary: "Backend error.",
							Details: "The backend had problems trying to add the subscription, please try again.",
						},
					))))).ServeHTTP(res, req.WithContext(ctx))
				return
			}
			maps.Copy(createResults, matchResults)

			resp = models.NewResponse(
				models.WithResponseTemplate(pages.ImportResults(createResults)),
			)
		}
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

func (a *API) getSubscriptionUnreadCounts(ctx context.Context, subscriptionMetadata models.SubscriptionMetadataSlice) (*aggregations.TermsAggregationResults, error) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.ErrUserCtx
	}

	if len(subscriptionMetadata) == 0 {
		return &aggregations.TermsAggregationResults{}, nil
	}

	subscriptionQueries := make([]query.Option, 0, len(subscriptionMetadata))
	for m := range slices.Values(subscriptionMetadata) {
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, m))
	}
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)

	aggResults, resp := a.DataAPI().ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(subscriptionMetadata)))
	if resp != nil && !resp.IsNotFound() {
		return nil, resp
	}
	var (
		categoryCounts aggregations.TermsAggregationResults
		err            error
	)
	if !resp.IsNotFound() {
		categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](aggResults.Aggregations, "UnreadCounts")
		if err != nil {
			return nil, fmt.Errorf("getSubscriptionUnreadCounts: %w", err)
		}
	}

	return &categoryCounts, nil
}

func (a *API) getSubscriptions(ctx context.Context, ids ...models.SubscriptionID) (models.SubscriptionsSlice, error) {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, ErrNoCtxData
	}
	allFavorites := user.GetFavorites().FilterByType(models.FavoriteTypeSubscription)
	// Get the subscription states.
	allMetadata := user.GetSubscriptionMetadata().FilterByIDs(ids...)
	// Get unread counts.
	unreadCounts, err := a.getSubscriptionUnreadCounts(ctx, allMetadata)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Get feed data for subscriptions.
	feeds, err := a.DataAPI().GetFeeds(ctx, allMetadata.GetFeedIDs()...)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Generate subscriptions from data sources.
	subscriptions := make(models.SubscriptionsSlice, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		var metadata *models.SubscriptionMetadata
		var count int
		if metadata = allMetadata.GetByFeedID(feed.GetID()); metadata == nil {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		if unreadCounts.HasResults() {
			count = unreadCounts.GetCount(feed.GetID())
		}

		subscription, err := models.GenerateSubscription(metadata, feed, count, allFavorites.HasFavorite(metadata.GetID()))
		if err != nil {
			slogctx.FromCtx(ctx).Warn("Could not generate subscription from data.",
				slog.Any("error", err),
			)
			continue
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

func (a *API) filterSubscriptions(ctx context.Context, filters *models.SubscriptionFilters) (models.SubscriptionsSlice, models.Pagination, error) {
	// Get subscriptions by ID.
	subscriptions, err := a.getSubscriptions(ctx, filters.Subscriptions...)
	if err != nil {
		return nil, "", fmt.Errorf("filterSubscriptions: %w", err)
	}
	// Filter subscriptions.
	sort := filters.GetSort()
	subscriptions = subscriptions.FilterByCategories(filters.Categories...).
		FilterByView(filters.View).
		Sort(&sort)
	// Set up pagination.
	var pagination string
	if filters.Pagination != "" {
		pagination = filters.Pagination
	}
	subscriptions, pagination = subscriptions.Paginate(pagination, filters.GetCount())
	return subscriptions, pagination, nil
}

func (a *API) markSubscriptions(ctx context.Context, request *models.MarkSubscriptionsRequest) error {
	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoCtxData
	}
	// Validate parameters.
	valid, err := request.Valid()
	if err != nil || !valid {
		return fmt.Errorf("markSubscriptions: %w", err)
	}
	// Mark user subscriptions.
	user.MarkSubscriptions(request.Mark, request.Subscriptions...)
	// Update the user.
	err = a.updateUser(ctx, map[string]any{
		"subscriptions": user.GetSubscriptionMetadata(),
	})
	if err != nil {
		return fmt.Errorf("markSubscriptions: %w", err)
	}
	return nil
}

type (
	addSubscriptionRequests map[*models.SubscriptionRequest]*models.Feed
)

// feedURLs retrieves the URLs from the subscription requests.
func (r addSubscriptionRequests) feedURLs() []string {
	urls := make([]string, 0, len(r))
	for req := range maps.Keys(r) {
		urls = append(urls, req.URL)
	}
	return urls
}

// matchFeedsToSubscriptionRequests takes a list of subscription requests, extracts the URLs in each and attempt to
// match them to existing feeds. Where there is no existing feed, it will attempt to generate new feed data. It then
// stores the subscriptions that need new feeds and any with existing feeds in the context for the next handler.
func (r addSubscriptionRequests) matchFeedsToSubscriptionRequests(ctx context.Context, api *API) (pages.AddSubscriptionResults, error) {
	// Extract user data.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", models.ErrUserCtx)
	}

	slogctx.FromCtx(ctx).Debug("Matching existing feeds to subscription requests...")

	// Paginate and gather all feeds matching the request URLs.
	var (
		feedPagination *models.Pagination
		existingFeeds  models.Feeds
	)
	for {
		count := 100
		feeds, nextResults, err := api.DataAPI().SearchFeeds(ctx, query.Terms("source_urls", r.feedURLs()...), count, nil, feedPagination)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}

		existingFeeds = append(existingFeeds, feeds...)

		if len(feeds) < count {
			break
		}
		feedPagination = &nextResults
	}

	results := make(pages.AddSubscriptionResults)
	feedsNeeded := make(addSubscriptionRequests)

	// Loop over existing feeds.
	for request := range r {
		existingFeed := existingFeeds.FindByURL(request.GetURL())
		switch {
		case existingFeed == nil: // No existing feed, create a new one.
			newFeed, err := models.NewFeedFromURL(ctx, request.GetURL())
			if err != nil {
				results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Unable to parse URL as a feed",
					Details: fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()),
				})
				continue
			}
			feedsNeeded[request] = newFeed
			slogctx.FromCtx(ctx).Debug("New feed needed for subscription.",
				slog.String("subscription", request.String()),
				slog.String("feed", newFeed.String()),
			)
		case user.IsSubscribedToFeed(existingFeed.GetID()): // User already subscribed, ignore request.
			results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "Already subscribed to " + existingFeed.GetTitle(),
			})
		default: // Existing feed.
			r[request] = existingFeed
			slogctx.FromCtx(ctx).Debug("Existing feed for subscription.",
				slog.String("subscription", request.String()),
				slog.String("feed", existingFeed.String()),
			)
		}
	}

	// Add new feeds for requests without an existing feed.
	if len(feedsNeeded) > 0 {
		newFeedsNeededResults, err := feedsNeeded.createNewFeeds(ctx, api)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}
		maps.Copy(r, feedsNeeded)
		maps.Copy(results, newFeedsNeededResults)
	}

	return results, nil
}

func (r addSubscriptionRequests) createNewFeeds(ctx context.Context, api *API) (pages.AddSubscriptionResults, error) {
	slogctx.FromCtx(ctx).Debug("Adding new feeds for subscriptions.")
	results := make(pages.AddSubscriptionResults)

	// Testing no-op.
	// return results, nil

	// Add the new feeds.
	index := elastic.FeedsIndexFromCtx(ctx)
	if index == "" {
		return nil, models.ErrNoUserCtx
	}
	addFeedsResults, err := elastic.BulkAdd(ctx, api.DataAPI(), index, slices.Collect(maps.Values(r))...)
	if err != nil && !errors.Is(err, bulk.ErrBulkHasErrors) {
		return nil, fmt.Errorf("createNewFeeds: %w", err)
	}

	// Process the add feed results.
	for request, feed := range r {
		resp, found := addFeedsResults[feed.GetID()]
		if found {
			if resp.Created() {
				// Success, add request to map of subscription needed.
				r[request] = feed
			} else {
				results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Internal Error. ",
					Details: "An internal, irrecoverable backend error occurred trying to add a subscription for the URL " + request.GetURL(),
				})
			}
		}
	}
	return results, nil
}

// AddSubscriptions handles adding new subscription via either the add or import user functionality. It
// handles: matching and filtering out requests against existing subscriptions, matching requests to existing feeds,
// creating new feeds as necessary and finally creating user subscriptions.
func (r addSubscriptionRequests) createNewSubscriptions(ctx context.Context, api *API) (pages.AddSubscriptionResults, error) {
	// Extract user data.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, fmt.Errorf("createNewSubscriptions: %w", models.ErrUserCtx)
	}

	slogctx.FromCtx(ctx).Debug("Adding new subscriptions.")

	// Loop through the subscriptions adding their state to the existing subscription states slice. For any
	// subscriptions that have customisation data, collect the customisation data for adding later.
	results := make(pages.AddSubscriptionResults)
	allMetadata := make(models.SubscriptionMetadataSlice, 0, len(r))
	for request, feed := range r {
		// // Ignore requests that have already got a message response, indicating some kind of failure or warning.
		// if r[request] != nil {
		// 	continue
		// }
		// Generate metadata and add to metadata slice.
		metadata := models.NewSubscriptionMetadata(user.GetID(), feed, request)
		valid, err := metadata.Valid()
		if err != nil || !valid {
			slogctx.FromCtx(ctx).Debug("Invalid subscription metadata.",
				slog.Any("error", err),
				slog.String("feed_id", feed.GetID()),
				slog.String("feed", feed.GetTitle()),
				slog.String("url", request.GetURL()),
			)
			results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Subscription creation failed",
				Details: request.GetURL(),
			})
			continue
		}
		allMetadata = append(allMetadata, metadata)
		// Generate subscription and add to results map.
		subscription, err := models.GenerateSubscription(metadata, feed, 0, false)
		if err != nil {
			results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Subscription creation failed",
				Details: request.GetURL(),
			})
			continue
		}
		results[request] = models.NewSubscriptionResult(subscription, &models.UserMessage{
			Status:  models.UserMessageStatusSuccess,
			Summary: "Subscription Created: " + results[request].Subscription.GetTitle(),
			Details: "Articles will be fetched shortly...",
		})
	}

	// Testing no-op.
	// return results, nil

	// Add the subscription states.
	user.AddSubscriptions(allMetadata...)
	// Update the user object.
	err := api.updateUser(ctx, map[string]any{
		"subscriptions": user.Subscriptions,
	})
	if err != nil {
		return nil, fmt.Errorf("createNewSubscriptions: %w", err)
	}
	return results, nil
}
