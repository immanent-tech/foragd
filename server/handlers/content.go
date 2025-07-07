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

	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/views"
)

// ParseNewSubscriptionRequest will extract the subscription request, validate it and then store it in the context for
// further processing.
func ParseNewSubscriptionRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
		if err != nil || !valid {
			ctx = context.WithValue(ctx, respCtxKey, models.NewResponse(http.StatusNotAcceptable, err))
		} else {
			// Create a map of requests to their individual results.
			results := make(map[*models.SubscriptionRequest]*models.UserMessage)
			ctx = context.WithValue(ctx, subscriptionResultsCtxKey, results)
		}
		ctx = context.WithValue(ctx, subscriptionRequestsCtxKey, request)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// NewSubscriptionRequestResult handles processing the result of an add subscription request.
func NewSubscriptionRequestResult(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// If there is a response object in the context, a backend error occurred and the add subscription request failed.
		_, found := req.Context().Value(respCtxKey).(*models.Response)
		if found {
			var request *models.SubscriptionRequest
			requests := subscriptionRequestsFromCtx(req.Context())
			if requests != nil {
				request = requests[0]
			}
			msg := &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "A problem occurred while adding the subscription.",
			}
			ctx := templateToCtx(req.Context(), views.NewSubscriptionModal(request, msg))
			next.ServeHTTP(res, req.WithContext(ctx))
		}
		// Extract the processed request from the context.
		results := subscriptionResultsFromCtx(req.Context())
		if results == nil {
			next.ServeHTTP(res, req)
			return
		}
		// Display the modal with the request results shown.
		for _, result := range results {
			ctx := templateToCtx(req.Context(), views.NewSubscriptionModal(&models.SubscriptionRequest{}, result))
			next.ServeHTTP(res, req.WithContext(ctx))
			break
		}
	})
}

// NewSubscriptionsImport handles setting up a new subscription import process for the user.
func NewSubscriptionsImport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := templateToCtx(req.Context(), views.ImportSubscriptionLayout())
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ProcessSubscriptionsImport handles both setting up the method and actioning the method used for the subscription
// import process.
func ProcessSubscriptionsImport(importMethod string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			switch req.Method {
			case http.MethodPut:
				switch importMethod {
				case "opml_file":
					ctx = templateToCtx(ctx, views.ImportFromOPML())
				}
			case http.MethodPost:
				switch importMethod {
				case "opml_file":
					// Decode the OPML file form input.
					opmlFile := &models.OPMLFile{}
					opmlFile, valid, err := forms.DecodeMultipartFile(req, "data", opmlFile)
					if err != nil || !valid {
						ctx = context.WithValue(ctx, respCtxKey,
							models.NewResponse(http.StatusNotAcceptable, fmt.Errorf("could not parse OPML file: %w", err)))
						return
					}
					opmlImport, err := opmlFile.Parse()
					if err != nil {
						ctx = context.WithValue(ctx, respCtxKey,
							models.NewResponse(http.StatusNotAcceptable, fmt.Errorf("could not parse OPML file: %w", err)))
						return
					}
					// Extract the individual feeds from the OPML object and create a subscription
					// request for each one.
					feeds := opmlImport.ExtractRSS()
					requests := make([]*models.SubscriptionRequest, 0, len(feeds))
					for _, feed := range feeds {
						requests = append(requests, &models.SubscriptionRequest{URL: feed.XMLURL})
					}
					// Create a map of requests to their individual results.
					results := make(map[*models.SubscriptionRequest]*models.Response)
					ctx = context.WithValue(ctx, subscriptionResultsCtxKey, results)
					// Store requests in context.
					ctx = context.WithValue(req.Context(), subscriptionRequestsCtxKey, requests)
				}
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SubscriptionsImportResults handles showing the result of a subscriptions import.
func SubscriptionsImportResults(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// If there is a response object in the context, a backend error occurred and the add subscription request failed.
		_, found := req.Context().Value(respCtxKey).(*models.Response)
		if found {
			msg := &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "A problem occurred while importing subscriptions.",
			}
			ctx := templateToCtx(req.Context(), views.NewSubscriptionModal(&models.SubscriptionRequest{}, msg))
			next.ServeHTTP(res, req.WithContext(ctx))
		}

		// Get the request.
		results := subscriptionResultsFromCtx(req.Context())
		if results == nil {
			next.ServeHTTP(res, req)
			return
		}
		ctx := templateToCtx(req.Context(), views.ImportResults(results))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// MatchFeedsToSubscriptionRequests takes a list of subscription requests, extracts the URLs in each and attempt to
// match them to existing feeds. Where there is no existing feed, it will attempt to generate new feed data. It then
// stores the subscriptions that need new feeds and any with existing feeds in the context for the next handler.
//
//nolint:funlen
func MatchFeedsToSubscriptionRequests(api models.DocumentsAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Skip processing if there is already a response in the context.
			if _, found := req.Context().Value(respCtxKey).(*models.Response); found {
				next.ServeHTTP(res, req)
				return
			}

			// Extract user data.
			user, found := models.UserFromCtx(req.Context())
			if !found {
				ctx := context.WithValue(req.Context(), respCtxKey, models.RespErrUnauthorized())
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}

			// Extract the requests from the context.
			requests := subscriptionRequestsFromCtx(req.Context())
			if requests == nil {
				slogctx.FromCtx(req.Context()).Debug("No subscription requests to process")
				next.ServeHTTP(res, req)
				return
			}

			slogctx.FromCtx(req.Context()).Debug("Matching existing feeds to subscription requests...")

			// Extract the results map from the context
			results := subscriptionResultsFromCtx(req.Context())
			// Get any existing feed that matches a request URL.
			feedURLs := make([]string, 0, len(requests))
			for request := range slices.Values(requests) {
				feedURLs = append(feedURLs, request.GetURL())
			}
			var (
				feedPagination *models.Pagination
				existingFeeds  models.Feeds
			)
			for {
				count := 100
				feeds, nextResults, err := api.SearchFeeds(req.Context(), query.Terms("source_url", feedURLs...), count, nil, feedPagination)
				if err != nil {
					ctx := context.WithValue(req.Context(), respCtxKey,
						models.NewResponse(http.StatusInternalServerError, fmt.Errorf("could not retrieve all feeds by URLs: %w", err)))
					next.ServeHTTP(res, req.WithContext(ctx))
					return
				}

				existingFeeds = append(existingFeeds, feeds...)

				if len(feeds) < count {
					break
				}
				feedPagination = &nextResults
			}

			newFeedsNeeded := make(map[*models.SubscriptionRequest]*models.Feed)
			newSubscriptions := make(map[*models.SubscriptionRequest]*models.Feed)
			// Loop over existing feeds.
			for request := range slices.Values(requests) { //nolint:contextcheck
				feed := existingFeeds.FindByURL(request.GetURL())
				switch {
				case feed == nil: // no existing feed, create a new one.
					newFeed, err := models.NewFeedFromURL(req.Context(), request.GetURL())
					if err != nil {
						results[request] = &models.UserMessage{
							Status:  models.UserMessageStatusError,
							Summary: "Unable to source feed details from request URL: " + request.GetURL(),
						}
						continue
					}
					newFeedsNeeded[request] = newFeed
					slogctx.FromCtx(req.Context()).Debug("New feed needed for subscription.",
						slog.String("subscription", request.String()),
						slog.String("feed", newFeed.String()),
					)
				case user.IsSubscribedToFeed(feed.GetID()): // user already subscribed, ignore request.
					results[request] = &models.UserMessage{
						Status:  models.UserMessageStatusWarning,
						Summary: "Already subscribed to feed with URL: " + request.GetURL(),
					}
				default: // existing feed.
					newSubscriptions[request] = feed
					slogctx.FromCtx(req.Context()).Debug("Existing feed for subscription.",
						slog.String("subscription", request.String()),
						slog.String("feed", feed.String()),
					)
				}
			}

			ctx := req.Context()
			// Add requests that need a feed created.
			ctx = context.WithValue(ctx, feedsCtxKey, newFeedsNeeded)
			// Add new subscriptions.
			ctx = context.WithValue(ctx, subscriptionsCtxKey, newSubscriptions)
			// Add any results.
			ctx = context.WithValue(ctx, subscriptionResultsCtxKey, results)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// AddFeedsForSubscriptionRequests takes a map of subscription requests that need new feeds and adds those feeds to the
// database. Successful adds are then added to the subscriptions that need creating while unsuccessful adds have their
// result recorded. This data is then passed to the next handler through the context.
func AddFeedsForSubscriptionRequests(api models.DocumentsAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Skip processing if there is already a response in the context.
			if _, found := req.Context().Value(respCtxKey).(*models.Response); found {
				next.ServeHTTP(res, req)
				return
			}

			// Get requests that need new feeds.
			newFeeds, ok := req.Context().Value(feedsCtxKey).(map[*models.SubscriptionRequest]*models.Feed)
			if !ok || len(newFeeds) == 0 {
				next.ServeHTTP(res, req)
				return
			}

			slogctx.FromCtx(req.Context()).Debug("Adding new feeds for subscription requests.")

			// Extract any existing results map from the context
			results := subscriptionResultsFromCtx(req.Context())
			// Extract new subscriptions from the context.
			subscriptions := subscriptionsNeededFromCtx(req.Context())

			// Add the new feeds.
			addFeedsResults, err := api.AddFeeds(req.Context(), slices.Collect(maps.Values(newFeeds))...)
			if err != nil {
				ctx := context.WithValue(req.Context(), respCtxKey,
					models.NewResponse(http.StatusInternalServerError, fmt.Errorf("add subscriptions failed: %w", err)))
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}
			// Process the add feed results.
			for request, feed := range newFeeds {
				resp, found := addFeedsResults[feed.GetID()]
				if found {
					if resp.Created() {
						// Success, add request to map of subscription needed.
						subscriptions[request] = feed
						continue
					}
				}
				results[request] = &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Unable to create a subscription for request URL: " + request.GetURL(),
				}
			}

			ctx := req.Context()
			// Add new subscriptions.
			ctx = context.WithValue(ctx, subscriptionsCtxKey, subscriptions)
			// Add any results.
			ctx = context.WithValue(ctx, subscriptionResultsCtxKey, results)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// AddSubscriptions handles adding new subscription via either the add or import user functionality. It
// handles: matching and filtering out requests against existing subscriptions, matching requests to existing feeds,
// creating new feeds as necessary and finally creating user subscriptions.
//
//nolint:funlen,gocognit // breaking up this function would actually add debugging/development complexity.
func AddSubscriptions(api models.DocumentsAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Skip processing if there is already a response in the context.
			if _, found := req.Context().Value(respCtxKey).(*models.Response); found {
				next.ServeHTTP(res, req)
				return
			}

			// Extract user data.
			user, found := models.UserFromCtx(req.Context())
			if !found {
				ctx := context.WithValue(req.Context(), respCtxKey, models.RespErrUnauthorized())
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}

			// Extract any existing results map from the context
			results := subscriptionResultsFromCtx(req.Context())
			// Extract new subscriptions from the context.
			subscriptions := subscriptionsNeededFromCtx(req.Context())
			if len(subscriptions) == 0 {
				next.ServeHTTP(res, req)
				return
			}
			// Get the existing subscription states from the user.
			newSubscriptions := make(map[models.SubscriptionID]*models.SubscriptionState)

			slogctx.FromCtx(req.Context()).Debug("Adding new subscriptions.")

			// Loop through the subscriptions adding their state to the existing subscription states slice. For any
			// subscriptions that have customisation data, collect the customisation data for adding later.
			customisations := make(models.SubscriptionCustomisations, 0, len(subscriptions))
			for request, feed := range subscriptions {
				// Add subscription state and mark successful.
				state := models.NewSubscriptionState(feed.GetID())
				newSubscriptions[state.GetID()] = state
				details := request.String()
				results[request] = &models.UserMessage{
					Status:  models.UserMessageStatusSuccess,
					Summary: "Subscription created!",
					Details: &details,
				}
				// Accrue any subscription customisations.
				if customisation := request.GenerateCustomisation(state.GetID(), user.GetID(), state.GetFeedID()); customisation != nil {
					customisations = append(customisations, customisation)
				}
			}

			if len(customisations) > 0 {
				// Add any subscription customisations. It's not critical if subscription customisation data is not added.
				customisationResults, err := api.AddSubscriptionCustomisations(req.Context(), customisations...)
				if err != nil {
					slogctx.FromCtx(req.Context()).Warn("Failed to add subscription customisations.",
						slog.Any("error", err),
					)
				}
				// Process the customisation results
				for _, resp := range customisationResults {
					if !resp.Created() {
						slogctx.FromCtx(req.Context()).Warn("Unable to create customisation.", slog.Any("error", resp))
						// for request := range subscriptions {
						// 	if request.SubscriptionID == id {
						// 		results[request] = &models.Response{
						// 			UserMessage: &models.UserMessage{
						// 				Status:  models.UserMessageStatusError,
						// 				Summary: "Unable to create a subscription for request URL: " + request.GetURL(),
						// 			},
						// 		}
						// 	}
						// }
					}
				}
			}
			// Update the user object.
			resp := api.UpdateUser(req.Context(), map[string]any{
				"subscriptions": slices.Collect(maps.Values(newSubscriptions)),
			})
			if resp != nil {
				ctx := context.WithValue(req.Context(), respCtxKey, resp)
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}
			// Store the results in the context for the next handler.
			ctx := context.WithValue(req.Context(), subscriptionResultsCtxKey, results)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func NewUserSignup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := templateToCtx(req.Context(), views.SignUpPage(models.NewUserSignup()))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func ProcessUserSignup(userBackendAPI models.UserBackendAPI, userFrontendAPI models.DocumentsAPI, signupRequest *models.UserSignupRequest) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Create the user in the auth backend.
			userID, err := userBackendAPI.Create(req.Context(), signupRequest)
			if err != nil {
				ProcessResponse(res, req,
					models.NewResponse(http.StatusInternalServerError, fmt.Errorf("failed to update user: %w", err)))
				return
			}
			// Create new user in the database backend.
			ctx := elastic.UserIndexToCtx(req.Context(), schema.UsersSchemaPrefix)
			err = userFrontendAPI.AddUser(ctx, userID)
			if err != nil {
				ProcessResponse(res, req,
					models.NewResponse(http.StatusInternalServerError, fmt.Errorf("failed to update user: %w", err)))
				return
			}
			signupRequest.Msg = &models.UserMessage{
				Status:  models.UserMessageStatusSuccess,
				Summary: "Account created!",
			}
			ctx = templateToCtx(ctx, views.SignupForm(signupRequest))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
