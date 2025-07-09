// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/aggregations"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/content"
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
			chain.Then(RenderTemplate(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		chain = chain.Append(SavePageState(filters))
		var template templ.Component
		// Retrieve user object.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			chain.Then(RenderTemplate(RespForbidden())).ServeHTTP(res, req)
			return
		}
		if len(user.GetSubscriptionsByID()) == 0 {
			template = views.EmptyContent()
		} else {
			// Get subscriptions matching filters.
			subscriptions, pagination, resp := a.filterSubscriptions(req.Context(), filters)
			if resp != nil {
				chain.Then(RenderTemplate(RespBackendError(err))).ServeHTTP(res, req)
				return
			}
			cards := make([]templ.Component, 0, len(subscriptions))
			states := make([]templ.Component, 0, len(subscriptions))
			for subscription := range slices.Values(subscriptions) {
				cards = append(cards, views.NewSubscriptionContent(subscription).Card())
				states = append(states, views.NewSubscriptionContent(subscription).State())
			}
			// Add pagination element if pagination is required.
			if pagination != "" && len(cards) == filters.GetCount() {
				// Add pagination htmx props to last article.
				cards = append(cards, content.PaginationControl(req.Context(), "/subscriptions", pagination))
			}
			// Generate page template.
			template = views.NewSubscriptionsPage(filters, subscriptions.GetCategoryCounts(), cards...).Show()
		}

		resp := models.NewResponse(
			models.WithResponseTemplate(template),
		)

		// Display content based on request.
		switch {
		case req.Method == http.MethodPost:
			// Pagination. Only render cards.
			chain.Then(RenderTemplateFragments(resp, "cards")).ServeHTTP(res, req)
		case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
			// Partial update. Only render fragments.
			chain.Then(RenderTemplateFragments(resp, "content-header", "content", "content-footer")).ServeHTTP(res, req)
		default:
			// Full page render.
			chain.Then(RenderTemplate(resp)).ServeHTTP(res, req)
		}
	}
}

// MarkSubscriptions handles marking a collection of subscriptions as read or unread.
func (a *API) MarkSubscriptions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
			TriggerStateUpdates,
		)
		// Get subscription details.
		request, valid, err := forms.DecodeForm[*models.MarkSubscriptionsRequest](req)
		if err != nil || !valid {
			chain.Then(RenderTemplate(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		chain = chain.Append(SetupRedirect(request.Redirect))
		// Get mark.
		mark := chi.URLParam(req, "mark")
		// Set marked at to current timestamp.
		markedAt := time.Now().UTC()
		// Get all user subscription states.
		states, err := a.getSubscriptionStates(req.Context())
		if err != nil {
			chain.Then(RenderTemplate(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		// Loop through given subscription IDs and update states.
		filteredStates := states.FilterByIDs(request.Subscriptions...)
		for state := range slices.Values(filteredStates) {
			state.Mark(models.Mark(mark), markedAt)
		}

		// Index updates.
		if err := a.updateSubscriptionStates(req.Context(), filteredStates); err != nil {
			chain.Then(RenderTemplate(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		chain.Then(RenderTemplate(nil)).ServeHTTP(res, req)
	}
}

// RemoveSubscriptions handles removing (unsubscribing from) a collection of subscriptions.
func (a *API) RemoveSubscriptions() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Set up the handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Get subscription details.
		request, valid, err := forms.DecodeForm[*models.RemoveSubscriptionsRequest](req)
		if err != nil || !valid {
			chain.Then(RenderTemplate(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		// Act according to user confirmation.
		var resp *models.Response
		switch request.Confirmation {
		case models.UserConfirmationYes:
			slogctx.FromCtx(req.Context()).Debug("Subscription removal confirmed.",
				slog.String("subscription_id", strings.Join(request.Subscriptions, ",")),
			)
			if resp := a.removeSubscriptions(req.Context(), request.Subscriptions...); resp != nil {
				chain.Then(RenderTemplate(RespBackendError(err))).ServeHTTP(res, req)
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
			chain = chain.Append(TriggerStateUpdates)
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
			parameters := map[string]string{
				"subscriptions": strings.Join(request.Subscriptions, ","),
				"confirmation":  "yes",
			}
			modal := partials.AskQuestion("Unsubscribe?", templ.Attributes{
				"hx-post": "/subscriptions/remove",
				"hx-vals": partials.GenerateHXVals(parameters),
				"hx-swap": "outerHTML",
			})
			resp = models.NewResponse(
				models.WithResponseTemplate(modal),
			)
		}
		chain.Then(RenderTemplate(resp)).ServeHTTP(res, req)
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
		// Retrieve subscription details.
		states, err := a.getSubscriptionStates(req.Context(), id)
		if err != nil {
			chain.Then(RenderTemplate(RespBackendError(err))).ServeHTTP(res, req)
			return
		}
		if len(states) == 0 {
			chain.Then(RenderTemplate(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		edit := &models.SubscriptionEdit{
			SubscriptionID: states[0].GetID(),
			Title:          states[0].Customisation.Title,
			Categories:     states[0].Customisation.Categories,
		}
		// Get top categories across items in subscription feed.
		var topItemCategories []models.Category
		categories, err := a.getItemTopCategories(req.Context(), states[0].GetFeedID())
		if err == nil {
			topItemCategories = categories
		}
		resp := models.NewResponse(
			models.WithResponseTemplate(views.EditSubscriptionModal(edit, topItemCategories, nil)),
		)
		chain.Then(RenderTemplate(resp)).ServeHTTP(res, req)
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
		edits, valid, err := forms.DecodeForm[*models.SubscriptionEdit](req)
		if err != nil || !valid {
			chain.Then(RenderTemplate(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		index := elastic.SubscriptionsIndexFromCtx(req.Context())
		if index == "" {
			chain.Then(RenderTemplate(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		updates := map[string]any{
			"title":      edits.Title,
			"categories": edits.Categories,
		}

		if err := elastic.UpdateDoc(req.Context(), a.DataAPI().GetAPI(), index, edits.SubscriptionID, updates); err != nil {
			chain.Then(RenderTemplate(RespBackendError(err))).ServeHTTP(res, req)
			return
		}

		msg := models.SuccessUserMessage("Subscription updated.", nil)
		// TODO: get new subscription details and update subscription card.
		// Display a notification acknowledging save.
		resp := models.NewResponse(
			models.WithResponseTemplate(partials.ShowNotification(msg)),
		)
		chain.Then(RenderTemplate(resp)).ServeHTTP(res, req)
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
		).Then(RenderTemplate(resp)).ServeHTTP(res, req)
	}
}

func (a *API) AddSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		chain := alice.New(
			RouteLogger,
		)

		request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
		if err != nil || !valid {
			chain.Then(RenderTemplate(RespInvalidInput(err))).ServeHTTP(res, req)
			return
		}
		requests := newSubscriptionRequests{
			request: &models.UserMessage{},
		}
		subscriptions, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a)
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Adding a subscription failed.",
				slog.Any("error", err),
			)
			requests[request] = &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "A problem occurred while adding the subscription.",
			}
		}
		if err := requests.createNewSubscriptions(req.Context(), a, subscriptions); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Adding a subscription failed.",
				slog.Any("error", err),
			)
			requests[request] = &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "A problem occurred while adding the subscription.",
			}
		}
		// Display the modal with the request results shown.
		resp := models.NewResponse(
			models.WithResponseTemplate(views.SubscriptionAddedModal(requests[request])),
		)
		chain.Append(TriggerStateUpdates).Then(RenderTemplate(resp)).ServeHTTP(res, req)
	}
}

func (a *API) getSubscriptionStates(ctx context.Context, ids ...models.SubscriptionID) (models.SubscriptionStates, error) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.ErrNoUserCtx
	}

	s := user.GetSubscriptionsByID(ids...)
	index := elastic.SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return nil, elastic.ErrFetchCtx
	}

	states, err := elastic.GetDocs[models.SubscriptionID, *models.SubscriptionState](ctx, a.DataAPI().GetAPI(), index, slices.Collect(maps.Keys(s))...)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Some subscriptions could not be extracted from docs.",
			slog.Any("warnings", err))
	}

	return states, nil
}

func (a *API) updateSubscriptionStates(ctx context.Context, states models.SubscriptionStates) error {
	index := elastic.SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return elastic.ErrFetchCtx
	}

	resp, err := elastic.BulkUpdate(ctx, a.DataAPI(), index, states...)
	if err != nil {
		return fmt.Errorf("updateSubscriptionStates: %w", err)
	}
	spew.Dump(resp)

	return nil
}

func (a *API) addSubscriptionStates(ctx context.Context, states models.SubscriptionStates) error {
	index := elastic.SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return elastic.ErrFetchCtx
	}

	resp, err := elastic.BulkAdd(ctx, a.DataAPI(), index, states...)
	if err != nil {
		return fmt.Errorf("updateSubscriptionStates: %w", err)
	}
	spew.Dump(resp)

	return nil
}

func (a *API) getSubscriptionUnreadCounts(ctx context.Context, states models.SubscriptionStates) (*aggregations.TermsAggregationResults, error) {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return nil, models.ErrUserCtx
	}

	subscriptionQueries := make([]query.Option, 0, len(states))
	for _, state := range states {
		subscriptionQueries = append(subscriptionQueries, queryUnreadItems(user, state))
	}
	query := query.Bool(
		query.Filter(
			query.Bool(
				query.Should(subscriptionQueries...),
			),
		),
	)
	aggResults, resp := a.DataAPI().ItemsAggregation(ctx, query, aggregations.NewTermsAggregation("UnreadCounts", "feed_id", len(states)))
	if resp != nil {
		return nil, resp
	}
	var (
		categoryCounts aggregations.TermsAggregationResults
		err            error
	)
	categoryCounts.StringTermsAggregate, err = aggregations.ExtractAggregation[*types.StringTermsAggregate](aggResults.Aggregations, "UnreadCounts")
	if err != nil {
		return nil, fmt.Errorf("getSubscriptionUnreadCounts: %w", err)
	}

	return &categoryCounts, nil
}

func (a *API) getSubscriptions(ctx context.Context, ids ...models.SubscriptionID) (models.Subscriptions, error) {
	// Get the subscription states.
	states, err := a.getSubscriptionStates(ctx, ids...)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}

	// Get unread counts.
	unreadCounts, err := a.getSubscriptionUnreadCounts(ctx, states)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Get feed data for subscriptions.
	feeds, err := a.DataAPI().GetFeeds(ctx, states.GetFeedIDs()...)
	if err != nil {
		return nil, fmt.Errorf("getSubscriptions: %w", err)
	}
	// Generate subscriptions from data sources.
	subscriptions := make(models.Subscriptions, 0, len(feeds))
	for feed := range slices.Values(feeds) {
		var state *models.SubscriptionState
		var count int
		if state := states.GetByFeedID(feed.GetID()); state == nil {
			slogctx.FromCtx(ctx).Warn("No subscription state for retrieved feed.",
				slog.String("feed_id", feed.GetID()),
			)
			continue
		}
		if unreadCounts != nil {
			count = unreadCounts.GetCount(feed.GetID())
		}

		subscription, err := models.GenerateSubscription(state, feed, count)
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

func (a *API) filterSubscriptions(ctx context.Context, filters *models.SubscriptionFilters) (models.Subscriptions, models.Pagination, error) {
	subscriptions, err := a.getSubscriptions(ctx, filters.Subscriptions...)
	if err != nil {
		return nil, "", fmt.Errorf("filterSubscriptions: %w", err)
	}
	sort := filters.GetSort()

	subscriptions = subscriptions.FilterByCategories(filters.Categories...).
		FilterByView(filters.View).
		Sort(&sort)

	var pagination string
	if filters.Pagination != nil {
		pagination = *filters.Pagination
	}
	subscriptions, pagination = subscriptions.Paginate(pagination, filters.GetCount())
	return subscriptions, pagination, nil
}

func (a *API) removeSubscriptions(ctx context.Context, ids ...models.SubscriptionID) error {
	// Retrieve user object.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return models.ErrUserCtx
	}
	// Remove the subscription states.
	index := elastic.SubscriptionsIndexFromCtx(ctx)
	if index == "" {
		return elastic.ErrFetchCtx
	}

	err := elastic.DeleteDocs(ctx, a.DataAPI().GetAPI(), index,
		query.Terms("subscription_id", ids...),
	)
	if err != nil {
		return fmt.Errorf("failed to delete subscription customisations: %w", err)
	}

	// Remove states for given subscriptions from user.
	states := user.GetSubscriptionsByID()
	for id := range states {
		if slices.Contains(ids, id) {
			delete(states, id)
		}
	}
	// Update the user.
	return a.updateUser(ctx, map[string]any{
		"subscriptions": slices.Collect(maps.Values(states)),
	})
}

type (
	newSubscriptionRequests map[*models.SubscriptionRequest]*models.UserMessage
	newSubscriptions        map[*models.SubscriptionRequest]*models.Feed
)

// feedURLs retrieves the URLs from the subscription requests.
func (r newSubscriptionRequests) feedURLs() []string {
	urls := make([]string, 0, len(r))
	for req := range maps.Keys(r) {
		urls = append(urls, req.URL)
	}
	return urls
}

// MatchFeedsToSubscriptionRequests takes a list of subscription requests, extracts the URLs in each and attempt to
// match them to existing feeds. Where there is no existing feed, it will attempt to generate new feed data. It then
// stores the subscriptions that need new feeds and any with existing feeds in the context for the next handler.
func (r newSubscriptionRequests) matchFeedsToSubscriptionRequests(ctx context.Context, api *API) (newSubscriptions, error) {
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
		feeds, nextResults, err := api.DataAPI().SearchFeeds(ctx, query.Terms("source_url", r.feedURLs()...), count, nil, feedPagination)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}

		existingFeeds = append(existingFeeds, feeds...)

		if len(feeds) < count {
			break
		}
		feedPagination = &nextResults
	}

	newFeedsNeeded := make(newSubscriptions)
	newSubscriptions := make(newSubscriptions)
	// Loop over existing feeds.
	for request := range maps.Keys(r) { //nolint:contextcheck
		feed := existingFeeds.FindByURL(request.GetURL())
		switch {
		case feed == nil: // no existing feed, create a new one.
			newFeed, err := models.NewFeedFromURL(ctx, request.GetURL())
			if err != nil {
				r[request] = &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Unable to source feed details from request URL: " + request.GetURL(),
				}
				continue
			}
			newFeedsNeeded[request] = newFeed
			slogctx.FromCtx(ctx).Debug("New feed needed for subscription.",
				slog.String("subscription", request.String()),
				slog.String("feed", newFeed.String()),
			)
		case user.IsSubscribedToFeed(feed.GetID()): // user already subscribed, ignore request.
			r[request] = &models.UserMessage{
				Status:  models.UserMessageStatusWarning,
				Summary: "Already subscribed to feed with URL: " + request.GetURL(),
			}
		default: // existing feed.
			newSubscriptions[request] = feed
			slogctx.FromCtx(ctx).Debug("Existing feed for subscription.",
				slog.String("subscription", request.String()),
				slog.String("feed", feed.String()),
			)
		}
	}

	// Add new feeds for requests without an existing feed.
	if len(newFeedsNeeded) > 0 {
		s, err := r.createNewFeeds(ctx, api, newFeedsNeeded)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}
		maps.Copy(newSubscriptions, s)
	}

	return newSubscriptions, nil
}

func (r newSubscriptionRequests) createNewFeeds(ctx context.Context, api *API, feedsNeeded newSubscriptions) (newSubscriptions, error) {
	slogctx.FromCtx(ctx).Debug("Adding new feeds for subscriptions.")

	// Add the new feeds.
	addFeedsResults, err := api.DataAPI().AddFeeds(ctx, slices.Collect(maps.Values(feedsNeeded))...)
	if err != nil {
		return nil, fmt.Errorf("createNewFeeds: %w", err)
	}

	newSubscriptions := make(newSubscriptions)
	// Process the add feed results.
	for request, feed := range feedsNeeded {
		resp, found := addFeedsResults[feed.GetID()]
		if found {
			if resp.Created() {
				// Success, add request to map of subscription needed.
				newSubscriptions[request] = feed
				continue
			}
		}
		r[request] = &models.UserMessage{
			Status:  models.UserMessageStatusError,
			Summary: "Unable to create a subscription for request URL: " + request.GetURL(),
		}
	}
	return newSubscriptions, nil
}

// AddSubscriptions handles adding new subscription via either the add or import user functionality. It
// handles: matching and filtering out requests against existing subscriptions, matching requests to existing feeds,
// creating new feeds as necessary and finally creating user subscriptions.
func (r newSubscriptionRequests) createNewSubscriptions(ctx context.Context, api *API, newSubscriptions newSubscriptions) error {
	// Extract user data.
	user, found := models.UserFromCtx(ctx)
	if !found {
		return fmt.Errorf("createNewSubscriptions: %w", models.ErrUserCtx)
	}

	slogctx.FromCtx(ctx).Debug("Adding new subscriptions.")

	// Loop through the subscriptions adding their state to the existing subscription states slice. For any
	// subscriptions that have customisation data, collect the customisation data for adding later.
	states := make(models.SubscriptionStates, 0, len(newSubscriptions))
	for request, feed := range newSubscriptions {
		// Create subscription state.
		state := models.NewSubscriptionState(user.GetID(), feed, request)
		states = append(states, state)
		// Add subscription details to user object.
		user.AddSubscription(state.SubscriptionID, state.FeedID)
		details := request.String()
		r[request] = &models.UserMessage{
			Status:  models.UserMessageStatusSuccess,
			Summary: "Subscription created!",
			Details: &details,
		}
	}
	// Add the subscription states.
	if err := api.addSubscriptionStates(ctx, states); err != nil {
		return fmt.Errorf("createNewSubscriptions: %w", err)
	}
	// Update the user object.
	resp := api.updateUser(ctx, map[string]any{
		"subscriptions": slices.Collect(maps.Values(newSubscriptions)),
	})
	if resp != nil {
		return fmt.Errorf("createNewSubscriptions: %w", resp.InternalError)
	}
	return nil
}
