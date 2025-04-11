// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

// ParseSubscriptionRequest will extract the subscription request, validate it and then store it in the context for
// further processing.
func ParseSubscriptionRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
		if err != nil {
			msg := models.NewMessage(
				"Error parsing form.",
				models.MessageStatusError,
				models.WithError(err))
			// Store messages in context.
			ctx = context.WithValue(ctx, messagesCtxKey, []*models.Message{msg})
		}
		if !valid {
			msg := models.NewMessage(
				"Details are invalid.",
				models.MessageStatusError,
				models.WithError(err))
			ctx = context.WithValue(ctx, messagesCtxKey, []*models.Message{msg})
		}
		// Store subscriptions in context.
		ctx = context.WithValue(ctx, subscriptionRequestsCtxKey, models.SubscriptionRequests{request})
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// MatchRequestsWithFeeds will generate subscriptions from requests where there is an existing feed and the user is not
// already subscribed.
func MatchRequestsWithFeeds(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			user, found := models.UserFromCtx(req.Context())
			if !found {
				InternalServerError(res, req, models.ErrUserCtx)
				return
			}
			requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
			if !found {
				slogctx.FromCtx(req.Context()).Warn("No subscription requests found.")
				next.ServeHTTP(res, req)
				return
			}
			// Create a slice to hold results.
			var results []*models.Message
			// Create a slice to hold new subscriptions.
			var subscriptions models.Subscriptions

			// Find any existing feeds by request URLs.
			existingFeeds, err := api.GetFeedsByURL(req.Context(), requests.URLs()...)
			if err != nil {
				InternalServerError(res, req, err)
				next.ServeHTTP(res, req)
			}
			// Loop over existing feeds.
			for feed := range slices.Values(existingFeeds) {
				// Ignore requests where the user is already subscribed to the feed.
				if user.IsSubscribed(feed.GetID()) {
					results = append(results,
						models.NewMessage(fmt.Sprintf("Already subscribed to %s (%s)", feed.GetTitle(), feed.GetSourceURL()), models.MessageStatusWarning))
					continue
				}
				// Add a new subscription for any existing feeds the user is not already subscribed to.
				if request := requests.FindByURL(feed.GetSourceURL()); request != nil {
					subscriptions = append(subscriptions, models.NewSubscription(request, feed))
				}
			}
			// Store subscriptions in context.
			ctx := context.WithValue(req.Context(), subscriptionsCtxKey, subscriptions)
			// Store messages in context.
			ctx = context.WithValue(ctx, messagesCtxKey, results)
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// CreateNewFeedsForRequests will create new feeds for any requests that do not have an existing feed.
func CreateNewFeedsForRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get the subscription requests.
		requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			slogctx.FromCtx(req.Context()).Warn("No subscription requests found.")
			next.ServeHTTP(res, req)
			return
		}
		// Get the existing subscriptions.
		subscriptions, _ := req.Context().Value(subscriptionsCtxKey).(models.Subscriptions)
		// Get the existing results.
		results, _ := req.Context().Value(messagesCtxKey).([]*models.Message)

		// Create a slice to hold new feeds.
		var newFeeds models.Feeds

		// Loop over requests without an existing subscription.
		for request := range models.FilterSlice(requests,
			func(v *models.SubscriptionRequest) bool {
				return subscriptions.FindByURL(v.GetURL()) == nil
			}) {
			// Create a new feed from the request.
			newFeed, err := models.NewFeedFromURL(req.Context(), request.GetURL())
			if err != nil {
				results = append(results,
					models.NewMessage("Could not create feed for "+request.GetURL(),
						models.MessageStatusWarning,
						models.WithError(err),
					),
				)
				continue
			}
			slogctx.FromCtx(req.Context()).Debug("New feed.",
				slog.String("feed_name", newFeed.GetTitle()),
				slog.String("feed_url", newFeed.GetSourceURL()),
			)
			// Add the new feed.
			newFeeds = append(newFeeds, newFeed)
		}
		// Store new feeds in context.
		ctx := context.WithValue(req.Context(), feedsCtxKey, newFeeds)
		// Store messages in context.
		ctx = context.WithValue(ctx, messagesCtxKey, results)
		// Pass control to next handler.
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// AddFeedsForRequests adds new feeds for subscriptions.
func AddFeedsForRequests(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			newFeeds, found := req.Context().Value(feedsCtxKey).(models.Feeds)
			if !found || len(newFeeds) == 0 {
				next.ServeHTTP(res, req)
				return
			}
			requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
			if !found {
				slogctx.FromCtx(req.Context()).Warn("No subscription requests found.")
				next.ServeHTTP(res, req)
				return
			}
			// Get the existing subscriptions.
			subscriptions, _ := req.Context().Value(subscriptionsCtxKey).(models.Subscriptions)
			// Get the existing results.
			results, _ := req.Context().Value(messagesCtxKey).([]*models.Message)

			for feed := range slices.Values(newFeeds) {
				request := requests.FindByURL(feed.GetSourceURL())
				subscriptions = append(subscriptions, models.NewSubscription(request, feed))
			}

			// // Add the new feeds.
			// newFeedsResp, err := api.AddFeeds(req.Context(), newFeeds...)
			// if err != nil {
			// 	// On request failure, add error messages for all new feeds and then pass control to next handler.
			// 	for feed := range slices.Values(newFeeds) {
			// 		results = append(results,
			// 			models.NewMessage("Could not create feed for "+feed.GetSourceURL(),
			// 				models.MessageStatusWarning,
			// 				models.WithError(err),
			// 			),
			// 		)
			// 	}
			// 	// Store messages in context.
			// 	ctx := context.WithValue(req.Context(), messagesCtxKey, results)
			// 	next.ServeHTTP(res, req.WithContext(ctx))
			// 	return
			// }
			// // Loop over the results
			// for result := range slices.Values(newFeedsResp.Responses) {
			// 	// Ignore unknown results.
			// 	if result.Id_ == nil {
			// 		slogctx.FromCtx(req.Context()).Warn("Unknown add feed result.", slog.Any("result", result))
			// 		continue
			// 	}
			// 	// Match feed to result, ignore results with no feed.
			// 	feed := newFeeds.FindByID(*result.Id_)
			// 	if feed == nil {
			// 		slogctx.FromCtx(req.Context()).Warn("Result with unmatched feed.", slog.Any("result", result))
			// 		continue
			// 	}
			// 	request := requests.FindByURL(feed.URL)
			// 	if request == nil {
			// 		slogctx.FromCtx(req.Context()).Warn("Result with unmatched request", slog.Any("result", result))
			// 		continue
			// 	}
			// 	if _, err := result.State(); err != nil {
			// 		results = append(results,
			// 			models.NewMessage("Add subscription failed.", models.MessageStatusError, models.WithError(err)),
			// 		)
			// 	} else {
			// 		subscriptions = append(subscriptions, models.NewSubscription(request, feed))
			// 	}
			// }

			// Update stored subscriptions.
			ctx := context.WithValue(req.Context(), subscriptionsCtxKey, subscriptions)
			// Update stored messages.
			ctx = context.WithValue(ctx, messagesCtxKey, results)
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// AddSubscriptionsForRequests will add subscriptions for all requests.
func AddSubscriptionsForRequests(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get the existing subscriptions.
			subscriptions, found := req.Context().Value(subscriptionsCtxKey).(models.Subscriptions)
			if !found || len(subscriptions) == 0 {
				next.ServeHTTP(res, req)
				return
			}
			// Get the existing results.
			results, _ := req.Context().Value(messagesCtxKey).([]*models.Message)
			// Validate subscriptions.
			subscriptions = slices.Collect(models.FilterSlice(subscriptions,
				func(v *models.Subscription) bool {
					if valid, err := v.Valid(); !valid || err != nil {
						results = append(results,
							models.NewMessage(
								"Add subscription failed.",
								models.MessageStatusError,
								models.WithError(err)),
						)
						return false
					}
					return true
				}),
			)

			for sub := range slices.Values(subscriptions) {
				results = append(results,
					models.NewMessage(
						fmt.Sprintf("Added subscription! %s", sub.GetName()),
						models.MessageStatusSuccess),
				)
			}

			// // Add valid subscriptions.
			// err := api.AddSubscriptions(req.Context(), subscriptions)
			// // If the request to add subscriptions failed, record failure for all subscriptions.
			// for sub := range slices.Values(subscriptions) {
			// 	if err != nil {
			// 		results = append(results,
			// 			models.NewMessage(
			// 				"Add subscription failed.",
			// 				models.MessageStatusError,
			// 				models.WithError(err)),
			// 		)
			// 	} else {
			// 		results = append(results,
			// 			models.NewMessage(
			// 				fmt.Sprintf("Subscription for feed %s created!", sub.GetName()),
			// 				models.MessageStatusSuccess),
			// 		)
			// 	}
			// }

			// Update stored messages.
			ctx := context.WithValue(req.Context(), messagesCtxKey, results)
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func SubscriptionResponse() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get the request.
		requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			InternalServerError(res, req, errors.New("no subscription request"))
		}
		// Get the results.
		results, _ := req.Context().Value(messagesCtxKey).([]*models.Message)
		// Create a new response writer.
		resp := htmx.NewResponse()
		HTMXResponse(resp, subscription.NewSubscriptionRequest(requests[0]).Form(results[0])).ServeHTTP(res, req)
	})
}
