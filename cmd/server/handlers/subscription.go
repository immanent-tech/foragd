// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

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
			request.Result = models.NewMessage(
				"Error parsing form.",
				models.MessageStatusError,
				models.WithError(err))
		}
		if !valid {
			request.Result = models.NewMessage(
				"Details are invalid.",
				models.MessageStatusError,
				models.WithError(err))
		}
		// Store subscriptions in context.
		ctx = context.WithValue(ctx, subscriptionRequestsCtxKey, models.SubscriptionRequests{request})
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ParseOPMLFromFile will parse an OPML file that has been uploaded via the request and generate subscription requests
// from it.
func ParseOPMLFromFile(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode the OPML file form input.
		opmlFile := &models.OPMLFile{}
		opmlFile, valid, err := forms.DecodeMultipartFile(req, "data", opmlFile)
		if err != nil || !valid {
			ImportResults(models.NewMessage(
				"Could not parse OPML file.",
				models.MessageStatusError,
				models.WithError(err)),
			).ServeHTTP(res, req)
			return
		}
		opmlImport, err := opmlFile.Parse()
		if err != nil {
			ImportResults(models.NewMessage(
				"Could not parse OPML file.",
				models.MessageStatusError,
				models.WithError(err)),
			).ServeHTTP(res, req)
			return
		}
		// Extract the individual feeds from the OPML object and create a subscription
		// request for each one.
		feeds := opmlImport.ExtractRSS()
		requests := make(models.SubscriptionRequests, 0, len(feeds))
		for _, feed := range feeds {
			requests = append(requests, models.NewSubscriptionRequest(feed.XMLURL))
		}
		// Store requests in context.
		ctx := context.WithValue(req.Context(), subscriptionRequestsCtxKey, requests)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ProcessImportMethod will parse which import method has been chosen from the request, then call the appropriate
// handler for handling that type of import.
func ProcessImportMethod(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode the import source.
		importMethod, err := forms.DecodeMultipartValue(req, "source")
		if err != nil {
			ImportResults(models.NewMessage(
				"Error processing OPML import.",
				models.MessageStatusError,
				models.WithError(err)),
			).ServeHTTP(res, req)
			return
		}
		// Generate subscription requests using the import source.
		switch importMethod {
		case string(models.ImportSourceOPMLFile):
			slogctx.FromCtx(req.Context()).Debug("Starting import from OPML file.")
			next = ParseOPMLFromFile(next)
		}
		next.ServeHTTP(res, req)
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
			slogctx.FromCtx(req.Context()).Debug("Matching subscription requests to existing feeds.")
			// Find any existing feeds by request URLs.
			existingFeeds, err := api.GetFeedsByURL(req.Context(), requests.URLs()...)
			if err != nil {
				ImportResults(models.NewMessage(
					"Could not process request.",
					models.MessageStatusError,
					models.WithError(err)),
				).ServeHTTP(res, req)
				return
			}
			// Loop over existing feeds.
			for request := range slices.Values(requests) { //nolint:contextcheck
				feed := existingFeeds.FindByURL(request.GetURL())
				if feed == nil {
					// No existing feed matches request, will add new feed in next handler.
					continue
				}
				if user.IsSubscribed(feed.GetID()) {
					// Ignore requests where the user is already subscribed to the feed.
					request.Result = models.NewMessage("Already subscribed.",
						models.MessageStatusWarning,
						models.WithDetails(
							fmt.Sprintf("A subscription for %s (%s) already exists.", feed.GetTitle(), feed.GetSourceURL())),
					)
					slogctx.FromCtx(req.Context()).Debug("Already subscribed.",
						slog.String("subscription_nickname", request.UserNickname),
						slog.String("feed_id", feed.GetID()),
						slog.String("feed_name", feed.GetTitle()),
						slog.String("source_url", request.GetURL()),
					)
					continue
				}
				// Attach the subscription to the request.
				request.Subscription = models.NewSubscription(request, feed)
				slogctx.FromCtx(req.Context()).Debug("New subscription (existing feed).",
					slog.String("subscription_id", request.GetID()),
					slog.String("subscription_nickname", request.UserNickname),
					slog.String("feed_id", feed.GetID()),
					slog.String("feed_name", feed.GetTitle()),
					slog.String("source_url", request.GetURL()),
				)
			}
			// Store requests in context.
			ctx := context.WithValue(req.Context(), subscriptionRequestsCtxKey, requests)
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
		slogctx.FromCtx(req.Context()).Debug("Creating new feeds for subscription requests.")
		// Create a slice to hold new feeds.
		var newFeeds models.Feeds
		// Loop over requests without an existing subscription and no result.
		for request := range slices.Values(requests.FilterNoSubscription().FilterNoResults()) { //nolint:contextcheck
			// Create a new feed from the request.
			newFeed, err := models.NewFeedFromURL(req.Context(), request.GetURL())
			if err != nil {
				request.Result = models.NewMessage("Add subscription failed.",
					models.MessageStatusError,
					models.WithDetails(fmt.Sprintf("Could not create a feed for %s: %v", request.GetURL(), err)),
					models.WithError(err),
				)
				slogctx.FromCtx(req.Context()).Debug("Create feed failed.",
					slog.String("subscription_nickname", request.UserNickname),
					slog.String("source_url", request.GetURL()),
					slog.Any("error", err),
				)
				continue
			}
			// Add the new feed.
			newFeeds = append(newFeeds, newFeed)
			// Update the request URL (in case of redirection).
			request.URL = newFeed.GetSourceURL()
			// Create a subscription.
			request.Subscription = models.NewSubscription(request, newFeed)
			slogctx.FromCtx(req.Context()).Debug("New subscription (new feed).",
				slog.String("subscription_id", request.GetID()),
				slog.String("subscription_nickname", request.UserNickname),
				slog.String("feed_id", newFeed.GetID()),
				slog.String("feed_name", newFeed.GetTitle()),
				slog.String("source_url", request.GetURL()),
			)
		}
		// Store new feeds in context.
		ctx := context.WithValue(req.Context(), feedsCtxKey, newFeeds)
		// Store requests in context.
		ctx = context.WithValue(ctx, subscriptionRequestsCtxKey, requests)
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
			slogctx.FromCtx(req.Context()).Debug("Adding feeds for subscription requests.")
			for feed := range slices.Values(newFeeds) { //nolint:contextcheck
				request := requests.FilterNoResults().FindByURL(feed.GetSourceURL())
				if request == nil {
					slogctx.FromCtx(req.Context()).Warn("New feed but could not find matching request.",
						slog.Any("feed", feed))
					continue
				}
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

			// Store requests in context.
			ctx := context.WithValue(req.Context(), subscriptionRequestsCtxKey, requests)
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// AddSubscriptionsForRequests will add subscriptions for all requests.
func AddSubscriptionsForRequests(api DataAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Get the requests.
			requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
			if !found || len(requests) == 0 {
				next.ServeHTTP(res, req)
				return
			}
			// Validate subscriptions.
			validRequests := requests.FilterValid()
			for request := range slices.Values(validRequests) {
				request.Result = models.NewMessage(
					fmt.Sprintf("Added subscription: %s (%s)",
						request.Subscription.GetTitle(),
						request.Subscription.GetSourceURL(),
					),
					models.MessageStatusSuccess)
			}
			slogctx.FromCtx(req.Context()).Debug("Adding subscriptions.")

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
			// Store requests in context.
			ctx := context.WithValue(req.Context(), subscriptionRequestsCtxKey, requests)
			// Pass control to next handler.
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// AddSubscriptionResults handles showing the result of adding a new subscription.
func AddSubscriptionResults() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get the request.
		requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			InternalServerError(res, req, ErrMissingRequestData)
		}
		// Create a new response writer.
		resp := htmx.NewResponse()
		HTMXResponse(resp, subscription.NewSubscriptionRequest(requests[0]).Form(requests[0].Result)).ServeHTTP(res, req)
	})
}

// ImportResults handles showing the result of an import.
func ImportResults(err *models.Message) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if err != nil {
			if err := htmx.NewResponse().
				Retarget(subscription.ImportModalID.Target()).
				Reswap(htmx.SwapOuterHTML).
				RenderTempl(req.Context(), res,
					subscription.ImportResultsModal(subscription.ImportFailed(err)),
				); err != nil {
				InternalServerError(res, req, err)
			}
			return
		}
		// Get the request.
		requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			InternalServerError(res, req, ErrMissingRequestData)
		}
		// Generate a csv file containing all subscription request results.
		var resultsFile strings.Builder
		fmt.Fprintf(&resultsFile, `<script id="resultscsv" type="text/csv">`)
		fmt.Fprintf(&resultsFile, "status,summary,details\n")
		for request := range slices.Values(requests) {
			fmt.Fprintf(&resultsFile, request.Result.CSVString())
		}
		fmt.Fprintf(&resultsFile, "</script>")

		numSuccess := len(requests.FilterByStatus(models.MessageStatusSuccess))
		numFail := len(requests) - numSuccess

		if err := htmx.NewResponse().
			Retarget(subscription.ImportModalID.Target()).
			Reswap(htmx.SwapOuterHTML).
			RenderTempl(req.Context(), res,
				subscription.ImportResultsModal(subscription.ImportSuccess(numSuccess, numFail, resultsFile.String())),
			); err != nil {
			InternalServerError(res, req, err)
		}
	})
}
