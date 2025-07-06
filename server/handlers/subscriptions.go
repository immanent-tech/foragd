// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
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
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/views"
	"github.com/joshuar/go-feed-me/web/templates/content"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

// GetSubscriptions handles showing a filtered collection of subscriptions as cards.
func GetSubscriptions(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		filters, valid, err := forms.DecodeForm[*models.SubscriptionFilters](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
			return
		}
		// Get subscriptions matching filters.
		subscriptions, pagination, resp := models.FilterSubscriptions(req.Context(), api, filters)
		if resp != nil {
			RenderError(res, req, resp)
			return
		}
		cards := make([]templ.Component, 0, len(subscriptions))
		states := make([]templ.Component, 0, len(subscriptions))
		for subscription := range slices.Values(subscriptions) {
			cards = append(cards, content.NewSubscriptionContent(subscription).Card())
			states = append(states, content.NewSubscriptionContent(subscription).State())
		}
		// Add pagination element if pagination is required.
		if pagination != "" && len(cards) == filters.GetCount() {
			// Add pagination htmx props to last article.
			cards = append(cards, content.PaginationControl(req.Context(), "/subscriptions", pagination))
		}
		// Generate page template.
		subscriptionsPage := views.NewSubscriptionsPage(filters, subscriptions.GetCategoryCounts(), cards...)
		ctx := templateToCtx(req.Context(), subscriptionsPage.Show())
		// Set up handler chain.
		chain := alice.New(
			RouteLogger,
			SavePageState(filters),
		)
		// Display content based on request.
		switch {
		case req.Method == http.MethodPost:
			// Pagination. Only render cards.
			chain.Then(RenderTemplateFragments("cards")).ServeHTTP(res, req.WithContext(ctx))
		case htmx.IsHTMX(req) && !htmx.IsHistoryRestoreRequest(req):
			// Partial update. Only render fragments.
			chain.Then(RenderTemplateFragments("content-header", "content", "content-footer")).ServeHTTP(res, req.WithContext(ctx))
		default:
			// Full page render.
			chain.Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
		}
	}
}

// MarkSubscriptions handles marking a collection of subscriptions as read or unread.
func MarkSubscriptions(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Get subscription details.
		request, valid, err := forms.DecodeForm[*models.MarkSubscriptionsRequest](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
			return
		}
		// Get mark.
		mark := chi.URLParam(req, "mark")
		// Retrieve user.
		user, found := models.UserFromCtx(req.Context())
		if !found {
			RenderError(res, req, models.RespErrUnauthorized())
			return
		}
		// Set marked at to current timestamp.
		markedAt := time.Now().UTC()
		// Get all user subscription states.
		states := user.GetAllSubscriptionStates()
		// Loop through given subscription IDs and update states.
		for id := range slices.Values(request.Subscriptions) {
			if state, found := states[id]; !found {
				slogctx.FromCtx(req.Context()).Warn("Trying to mark non-existent user subscription.",
					slog.String("subscription_id", id),
				)
				continue
			} else {
				state.Mark(models.Mark(mark), markedAt)
			}
		}
		// Update the user object.
		if err := api.UpdateUser(req.Context(), map[string]any{
			"subscriptions": slices.Collect(maps.Values(states)),
		}); err != nil {
			RenderError(res, req, models.NewResponse(http.StatusInternalServerError, fmt.Errorf("could not process mark request: %w", err)))
			return
		}

		alice.New(
			RouteLogger,
			SetupRedirect(request.Redirect),
			TriggerStateUpdates,
		).Then(RenderTemplate()).ServeHTTP(res, req)
	}
}

// RemoveSubscriptions handles removing (unsubscribing from) a collection of subscriptions.
func RemoveSubscriptions(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Get subscription details.
		request, valid, err := forms.DecodeForm[*models.RemoveSubscriptionsRequest](req)
		if err != nil || !valid {
			RenderError(res, req, models.NewResponse(http.StatusBadRequest, err))
			return
		}
		// Set up the handler chain.
		chain := alice.New(
			RouteLogger,
		)
		// Act according to user confirmation.
		ctx := req.Context()
		switch request.Confirmation {
		case models.UserConfirmationYes:
			slogctx.FromCtx(ctx).Debug("Subscription removal confirmed.",
				slog.String("subscription_id", strings.Join(request.Subscriptions, ",")),
			)
			if resp := models.Unsubscribe(ctx, api, request.Subscriptions...); resp != nil {
				RenderError(res, req.WithContext(ctx), resp)
				return
			}
			// Show success notification.
			msg := &models.UserMessage{
				Summary: "Unsubscribed.",
				Status:  models.UserMessageStatusSuccess,
			}
			ctx = templateToCtx(ctx, partials.ShowNotification(msg))
			// Trigger state updates.
			chain = chain.Append(TriggerStateUpdates)
		case models.UserConfirmationCancel:
			slogctx.FromCtx(ctx).Debug("Subscription removal cancelled.",
				slog.String("subscription_id", strings.Join(request.Subscriptions, ",")),
			)
			// Don't swap any main content for user cancellation.
			// Display a notification acknowledging cancellation of request.
			msg := &models.UserMessage{
				Summary: "Request cancelled.",
				Status:  models.UserMessageStatusInfo,
			}
			ctx = templateToCtx(ctx, partials.ShowNotification(msg))
		default:
			slogctx.FromCtx(ctx).Debug("Confirming subscription removal.",
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
			ctx = templateToCtx(ctx, modal)
		}
		chain.Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
	}
}

// NewSubscription handles presenting the user with a form to enter details for adding a new subscription.
func NewSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		ctx := templateToCtx(req.Context(), views.NewSubscriptionModal(&models.SubscriptionRequest{}, nil))
		alice.New(
			RouteLogger,
		).Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
	}
}

// EditSubscription handles fetching and presenting the customisation data for a subscription, for the user to edit.
func EditSubscription(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "subscription")
		// Retrieve subscription customisation.
		customisation, resp := models.GetSubscriptionCustomisation(req.Context(), api, id)
		spew.Dump(customisation, resp)
		if resp != nil && !resp.IsNotFound() {
			RenderError(res, req, resp)
			return
		}
		edit := &models.SubscriptionEdit{
			SubscriptionID: customisation.GetID(),
			Title:          customisation.Title,
			Categories:     customisation.Categories,
		}
		// Get top categories across items in subscription feed.
		var topItemCategories []models.Category
		categories, resp := getItemTopCategories(req.Context(), api, customisation.GetFeedID())
		if resp == nil {
			topItemCategories = categories
		}
		ctx := templateToCtx(req.Context(), views.EditSubscriptionModal(edit, topItemCategories, nil))
		alice.New(
			RouteLogger,
		).Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
	}
}

// SaveSubscription handles saving edits made to a subscription by the user.
func SaveSubscription(api models.DocumentsAPI) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		edits, valid, err := forms.DecodeForm[*models.SubscriptionEdit](req)
		if err != nil || !valid {
			RenderError(res, req, models.RespErrBackend(err))
			return
		}
		var msg *models.UserMessage
		ctx := req.Context()
		if err := api.UpdateSubscriptionCustomisation(ctx, edits); err != nil {
			RenderError(res, req,
				models.NewResponse(http.StatusInternalServerError, fmt.Errorf("failed to update user: %w", err)))
			// msg = models.FailedUserMessage("Failed to update the subscription.", nil)
			return
		}
		msg = models.SuccessUserMessage("Subscription updated.", nil)
		// TODO: get new subscription details and update subscription card.
		// Display a notification acknowledging save.
		ctx = templateToCtx(ctx, partials.ShowNotification(msg))
		alice.New(
			RouteLogger,
			TriggerStateUpdates,
		).Then(RenderTemplate()).ServeHTTP(res, req.WithContext(ctx))
	}
}
