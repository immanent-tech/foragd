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
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
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
		if valid, err := validation.ValidateVariable(id, "required,startswith=sub_"); !valid || err != nil {
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

// MarkSubscriptions handles marking a collection of subscriptions as read or unread.
func (a *API) MarkSubscriptions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
			TriggerStateUpdates,
		)
		// Retrieve user object.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		} // Get subscription details.
		request, valid, err := forms.DecodeForm[*models.MarkSubscriptionsRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		chain = chain.Append(SetupRedirect(&request.Redirect))
		// Get mark.
		mark := chi.URLParam(req, "mark")
		// Mark user subscriptions.
		user.MarkSubscriptions(models.Mark(mark), request.Subscriptions...)
		// Update the user.
		err = a.updateUser(req.Context(), map[string]any{
			"subscriptions": user.GetSubscriptionMetadata(),
		})
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		chain.Then(RenderResponse(nil)).ServeHTTP(res, req)
	}
}

// RemoveSubscriptions handles removing (unsubscribing from) a collection of subscriptions.
func (a *API) RemoveSubscriptions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up the handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Retrieve user object.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		} // Get subscription details.
		request, valid, err := forms.DecodeForm[*models.RemoveSubscriptionsRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Act according to user confirmation.
		var resp *models.Response
		switch request.Confirmation {
		case models.UserConfirmationYes:
			slogctx.FromCtx(req.Context()).Debug("Subscription removal confirmed.",
				slog.String("subscription_id", strings.Join(request.Subscriptions, ",")),
			)
			// Remove metadata for given subscriptions from user.
			user.RemoveSubscriptions(request.Subscriptions...)
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
			resp = models.NewResponse(
				models.WithResponseTemplate(partials.ShowNotification(msg)),
			)
			// Trigger state updates.
			// chain = chain.Append(TriggerStateUpdates)
		case models.UserConfirmationCancel:
			slogctx.FromCtx(req.Context()).Debug("Subscription removal cancelled.",
				slog.String("subscription_id", strings.Join(request.Subscriptions, ",")),
			)
			// Don't swap any main content for user cancellation.
			// Display a notification acknowledging cancellation of request.
			msg := &models.UserMessage{
				Summary: "Request cancelled.",
				Status:  models.UserMessageStatusInfo,
			}
			resp = models.NewResponse(
				models.WithResponseTemplate(partials.ShowNotification(msg)),
			)
		default:
			slogctx.FromCtx(req.Context()).Debug("Confirming subscription removal.",
				slog.String("subscription_id", strings.Join(request.Subscriptions, ",")),
			)
			swapTargets := make([]string, 0, len(request.Subscriptions))
			for sub := range slices.Values(request.Subscriptions) {
				swapTargets = append(swapTargets, "#"+sub)
			}
			parameters := map[string]string{
				"subscriptions": strings.Join(request.Subscriptions, ","),
				"confirmation":  "yes",
			}
			modal := partials.AskQuestion("Unsubscribe?", templ.Attributes{
				"hx-post":   "/subscriptions/remove",
				"hx-vals":   partials.GenerateHXVals(parameters),
				"hx-target": "#" + request.Subscriptions[0],
				"hx-swap":   "outerHTML",
			})
			resp = models.NewResponse(
				models.WithResponseTemplate(modal),
			)
		}
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
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
		resp = models.NewResponse(
			models.WithResponseTemplate(views.EditSubscriptionModal(edit, topItemCategories, nil)),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// SaveSubscription handles saving edits made to a subscription by the user.
func (a *API) SaveSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
			TriggerStateUpdates,
		)
		// Retrieve user object.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderResponse(RespForbidden())).ServeHTTP(res, req)
			return
		}
		edits, valid, err := forms.DecodeForm[*models.SubscriptionEdit](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		metadata := user.GetSubscriptionMetadata().GetByID(edits.SubscriptionID)
		metadata.Customisation.Title = edits.Title
		metadata.Customisation.Categories = edits.Categories
		err = user.UpdateSubscription(metadata)
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Update the user.
		err = a.updateUser(req.Context(), map[string]any{
			"subscriptions": user.GetSubscriptionMetadata(),
		})
		if err != nil {
			chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		msg := models.SuccessUserMessage("Subscription updated.", "")
		// TODO: get new subscription details and update subscription card.
		// Display a notification acknowledging save.
		resp := models.NewResponse(
			models.WithResponseTemplate(partials.ShowNotification(msg)),
		)
		chain.Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// NewSubscription handles presenting the user with a form to enter details for adding a new subscription.
func NewSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		resp := models.NewResponse(
			models.WithResponseTemplate(views.NewSubscriptionModal(&models.SubscriptionRequest{})),
		)
		alice.New(
			RouteLogger,
		).Then(RenderResponse(resp)).ServeHTTP(res, req)
	}
}

// AddSubscription handles adding a new subscription requested by the user.
func (a *API) AddSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)
		request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
		if err != nil || !valid {
			chain.Then(RenderResponse(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		requests := addSubscriptionRequests{
			request: &models.Feed{},
		}
		// Match the request to either and existing or new feed.
		result, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a)
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Adding a subscription failed.",
				slog.Any("error", err),
			)
			resp := models.NewResponse(
				models.WithResponseTemplate(views.SubscriptionAddedModal(
					models.NewSubscriptionResult(nil, &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "A problem occurred while adding the subscription.",
					}),
				)),
			)
			chain.Append(TriggerStateUpdates).Then(RenderResponse(resp)).ServeHTTP(res, req)
			return
		}
		// If results returned from matching is non-nil, something went wrong.
		if result[request] != nil {
			resp := models.NewResponse(
				models.WithResponseTemplate(views.SubscriptionAddedModal(result[request])),
			)
			chain.Append(TriggerStateUpdates).Then(RenderResponse(resp)).ServeHTTP(res, req)
			return
		}
		// Create the new subscription.
		createResult, err := requests.createNewSubscriptions(req.Context(), a)
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Adding a subscription failed.",
				slog.Any("error", err),
			)
			result[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "A problem occurred while adding the subscription.",
			})
		} else {
			result = createResult
		}
		// Display the modal with the request results shown.
		resp := models.NewResponse(
			models.WithResponseTemplate(views.SubscriptionAddedModal(result[request])),
		)
		chain.Append(TriggerStateUpdates).Then(RenderResponse(resp)).ServeHTTP(res, req)
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
				models.WithResponseTemplate(views.ImportSubscriptionsModal()),
			)
		// PUT: set-up chosen import method.
		case http.MethodPut:
			resp = models.NewResponse(
				models.WithResponseTemplate(views.SetupOPMLImport()),
			)
		// POST: process import.
		case http.MethodPost:
			source := req.FormValue("source")
			slogctx.FromCtx(req.Context()).Debug("Starting import for user.",
				slog.String("source", source),
			)
			requests := make(addSubscriptionRequests)
			switch source {
			// Import via OPML file.
			case "opml_file":
				opmlFile := &models.OPMLFile{}
				opmlFile, valid, err := forms.DecodeMultipartFile(req, "data", opmlFile)
				if err != nil || !valid {
					chain.Then(RenderResponse(RespInvalidInput(fmt.Errorf("could not parse OPML file: %w", err)))).ServeHTTP(res, req)
					return
				}
				opmlImport, err := opmlFile.Parse()
				if err != nil {
					chain.Then(RenderResponse(RespInvalidInput(fmt.Errorf("could not parse OPML file: %w", err)))).ServeHTTP(res, req)
					return
				}
				feeds := opmlImport.ExtractRSS()
				for _, feed := range feeds {
					requests[&models.SubscriptionRequest{URL: feed.XMLURL}] = &models.Feed{}
				}
			}
			matchResults, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("User import failed.",
					slog.Any("error", err),
				)
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
			createResults, err := requests.createNewSubscriptions(req.Context(), a)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("User import failed.",
					slog.Any("error", err),
				)
				chain.Then(RenderResponse(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
			maps.Copy(createResults, matchResults)

			resp = models.NewResponse(
				models.WithResponseTemplate(views.ImportResults(createResults)),
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
func (r addSubscriptionRequests) matchFeedsToSubscriptionRequests(ctx context.Context, api *API) (views.AddSubscriptionResults, error) {
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

	results := make(views.AddSubscriptionResults)
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
					Details: fmt.Sprintf("The feed URL (%s) cannot be parsed as a feed source or is not a valid URL.", request.GetURL()),
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
				Summary: "Already subscribed.",
				Details: fmt.Sprintf("A subscription for the feed with URL %s already exists.", request.GetURL()),
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

func (r addSubscriptionRequests) createNewFeeds(ctx context.Context, api *API) (views.AddSubscriptionResults, error) {
	slogctx.FromCtx(ctx).Debug("Adding new feeds for subscriptions.")
	results := make(views.AddSubscriptionResults)

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
func (r addSubscriptionRequests) createNewSubscriptions(ctx context.Context, api *API) (views.AddSubscriptionResults, error) {
	// Extract user data.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, fmt.Errorf("createNewSubscriptions: %w", models.ErrUserCtx)
	}

	slogctx.FromCtx(ctx).Debug("Adding new subscriptions.")

	// Loop through the subscriptions adding their state to the existing subscription states slice. For any
	// subscriptions that have customisation data, collect the customisation data for adding later.
	results := make(views.AddSubscriptionResults)
	allMetadata := make(models.SubscriptionMetadataSlice, 0, len(r))
	for request, feed := range r {
		// Ignore requests that have already got a message response, indicating some kind of failure or warning.
		if results[request].Message != nil {
			continue
		}
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
			Summary: "Subscription Created",
			Details: fmt.Sprintf("Subscription for URL %s created.", request.GetURL()),
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
