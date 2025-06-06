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
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
	"github.com/joshuar/go-feed-me/web/views"
)

func GenerateArticleCollection(api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			articles, pagination, resp := getFilteredArticles(req.Context(), api, pagination, subIDs...)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}

			cards := views.GenerateArticleCards(req.Context(), articles, pagination)
			if len(cards) > 0 {
				cardLayout := partials.CardGrid(cards...)
				cardControls := partials.CardControls(
					views.RefreshAction(),
					views.UpdateSorting(),
					views.UpdateFilters(articles.GetItems().GetCategoryCounts()),
					views.CollectionActionsMenu(
						views.MarkAllArticlesAction(req.Context(), articles.GetSubscriptionIDs()...),
					),
				)

				ctx = context.WithValue(ctx, contentCtxKey, templ.Join(cardControls, cardLayout))
			} else {
				ctx = context.WithValue(ctx, contentCtxKey, views.EmptyContent())
			}
			ctx = context.WithValue(ctx, titleCtxKey, "Articles")
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateArticle handles displaying an item as an article.
func GenerateArticle(dataAPI DataAPI, itemID models.ItemID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			article, found, resp := dataAPI.GetArticle(req.Context(), itemID)
			if resp.IsError() || !found {
				ProcessResponse(res, req, resp)
				return
			}
			articleLayout := views.BuildArticleLayout(article)

			ctx := req.Context()
			ctx = context.WithValue(ctx, contentCtxKey, articleLayout)
			ctx = context.WithValue(ctx, titleCtxKey, article.Item.GetTitle())
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// FetchSubscriptions fetches subscriptions from the data backend and stores them in the request context for usage by other handlers.
func GenerateSubscriptionCollection(api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			subscriptions, pagination, resp := getFilteredSubscriptions(req.Context(), api, pagination, subIDs...)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}

			ctx := req.Context()
			cards := views.GenerateSubscriptionCards(req.Context(), pagination, subscriptions)
			if len(cards) > 0 {
				cardLayout := partials.CardGrid(cards...)
				cardControls := partials.CardControls(
					views.RefreshAction(),
					views.UpdateSorting(),
					views.UpdateFilters(subscriptions.GetCategoryCounts()),
					views.CollectionActionsMenu(
						views.MarkAllSubscriptionsAction(req.Context()),
					),
				)
				ctx = context.WithValue(req.Context(), contentCtxKey, templ.Join(cardControls, cardLayout))
			} else {
				ctx = context.WithValue(ctx, contentCtxKey, views.EmptyContent())
			}
			ctx = context.WithValue(ctx, titleCtxKey, "Subscriptions")
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func PaginateSubscriptionCollection(api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			subscriptions, pagination, resp := getFilteredSubscriptions(req.Context(), api, pagination, subIDs...)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			cards := views.GenerateSubscriptionCards(req.Context(), pagination, subscriptions)
			ctx := context.WithValue(req.Context(), contentCtxKey, templ.Join(cards...))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// ParseSubscriptionRequest will extract the subscription request, validate it and then store it in the context for
// further processing.
func ParseSubscriptionRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
		if err != nil {
			details := err.Error()
			request.Result = &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Unable to parse request.",
				Details: &details,
			}
		}
		if !valid {
			details := err.Error()
			request.Result = &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Unable to parse request.",
				Details: &details,
			}
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
			details := err.Error()
			ImportResults(&models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Could not parse OPML file.",
				Details: &details,
			}).ServeHTTP(res, req)
			return
		}
		opmlImport, err := opmlFile.Parse()
		if err != nil {
			details := err.Error()
			ImportResults(&models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Could not parse OPML file.",
				Details: &details,
			}).ServeHTTP(res, req)
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
			details := err.Error()
			ImportResults(&models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Error processing OPML import.",
				Details: &details,
			}).ServeHTTP(res, req)
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
				ProcessResponse(res, req, models.RespInvalidUser())
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
			// existingFeeds, err := api.GetFeedsByURL(req.Context(), requests.URLs()...)
			existingFeeds, err := api.FeedsSearchAll(req.Context(), query.URLs("source", requests.URLs()...))
			if err != nil {
				details := err.Error()
				ImportResults(&models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Could not process request.",
					Details: &details,
				}).ServeHTTP(res, req)
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
					details := "A subscription for " + feed.String() + " already exists."
					request.Result = &models.UserMessage{
						Status:  models.UserMessageStatusInfo,
						Summary: "Already Subscribed",
						Details: &details,
					}
					slogctx.FromCtx(req.Context()).Warn("Already subscribed.",
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
				details := err.Error()
				request.Result = &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Could not add " + request.String(),
					Details: &details,
				}
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
			// Add the new feeds.
			newFeedsResp, err := api.AddFeeds(req.Context(), newFeeds...)
			if err != nil {
				// On error, record all requests that need a new feed with an error.
				for feed := range slices.Values(newFeeds) {
					if request := requests.FilterNoResults().FindByURL(feed.GetSourceURL()); request == nil {
						slogctx.FromCtx(req.Context()).Warn("New feed but could not find matching request.",
							slog.Any("feed", feed))
						continue
					} else {
						details := err.Error()
						request.Result = &models.UserMessage{
							Status:  models.UserMessageStatusError,
							Summary: "Could not create feed for " + feed.GetSourceURL(),
							Details: &details,
						}
					}
				}
			}
			// Loop over the results
			for result := range slices.Values(newFeedsResp.Responses) {
				// Ignore unknown results.
				if result.Id_ == nil {
					slogctx.FromCtx(req.Context()).Warn("Unknown add feed result.", slog.Any("result", result))
					continue
				}
				// Match feed to result, ignore results with no feed.
				feed := newFeeds.FindByID(*result.Id_)
				if feed == nil {
					slogctx.FromCtx(req.Context()).Warn("Result with unmatched feed.", slog.Any("result", result))
					continue
				}
				// Get the request that matches the new feed.
				request := requests.FindByURL(feed.URL)
				if request == nil {
					slogctx.FromCtx(req.Context()).Warn("Result with unmatched request", slog.Any("result", result))
					continue
				}
				// If the new feed failed to be added, record an error against the request.
				if _, err := result.State(); err != nil {
					details := err.Error()
					request.Result = &models.UserMessage{
						Status:  models.UserMessageStatusError,
						Summary: "Add subscription failed.",
						Details: &details,
					}
				}
			}

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
			validRequests := requests.FilterValid().FilterWithSubscription()
			slogctx.FromCtx(req.Context()).Debug("Adding subscriptions.")
			// Add valid subscriptions.
			err := api.AddSubscriptions(req.Context(), validRequests.Subscriptions())
			if err != nil {
				for request := range slices.Values(validRequests) {
					request.Result = err.UserMessage
				}
			} else {
				for request := range slices.Values(validRequests) {
					request.Result = &models.UserMessage{
						Status:  models.UserMessageStatusSuccess,
						Summary: fmt.Sprintf("Subscription for feed %s created!", request.String()),
					}
				}
			}
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
		_, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			ProcessResponse(res, req, &models.Response{
				StatusCode: http.StatusNoContent,
				UserMessage: &models.UserMessage{
					Status:  models.UserMessageStatusWarning,
					Summary: "No response!",
				},
			})
		}
		// PartialRender(partials.ShowNotification(requests[0].Result)).ServeHTTP(res, req.WithContext(req.Context()))
	})
}

// ImportResults handles showing the result of an import.
func ImportResults(err *models.UserMessage) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if err != nil {
			if err := htmx.NewResponse().
				Retarget(subscription.ImportModalID.Target()).
				Reswap(htmx.SwapOuterHTML).
				RenderTempl(req.Context(), res,
					subscription.ImportResultsModal(subscription.ImportFailed(err)),
				); err != nil {
			}
			return
		}
		// Get the request.
		requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			ProcessResponse(res, req, &models.Response{
				StatusCode: http.StatusNoContent,
				UserMessage: &models.UserMessage{
					Status:  models.UserMessageStatusWarning,
					Summary: "No requests found!",
				},
			})
		}
		// Generate a csv file containing all subscription request results.
		var resultsFile strings.Builder
		fmt.Fprintf(&resultsFile, `<script id="resultscsv" type="text/csv">`)
		fmt.Fprintf(&resultsFile, "status,summary,details\n")
		for request := range slices.Values(requests) {
			fmt.Fprintf(&resultsFile, request.Result.CSVString())
		}
		fmt.Fprintf(&resultsFile, "</script>")

		numSuccess := len(requests.FilterByStatus(models.UserMessageStatusSuccess))
		numFail := len(requests) - numSuccess

		if err := htmx.NewResponse().
			Retarget(subscription.ImportModalID.Target()).
			Reswap(htmx.SwapOuterHTML).
			RenderTempl(req.Context(), res,
				subscription.ImportResultsModal(subscription.ImportSuccess(numSuccess, numFail, resultsFile.String())),
			); err != nil {
		}
	})
}

// RemoveSubscription handles processing a subscription removal request.
func RemoveSubscription(api UserAPI, subscriptionID models.SubscriptionID, confirmation models.UserConfirmation) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Add a new HTMX response writer to the context.
			ctx := req.Context()

			// Act according to user confirmation.
			switch confirmation {
			case models.UserConfirmationYes:
				slogctx.FromCtx(ctx).Debug("Subscription removal confirmed.",
					slog.String("subscription_id", subscriptionID),
				)
				if resp := removeSubscriptions(ctx, api, subscriptionID); resp.IsError() {
					ProcessResponse(res, req.WithContext(ctx), resp)
					return
				}
				// Show success notification.
				msg := &models.UserMessage{
					Summary: "Unsubscribed.",
					Status:  models.UserMessageStatusSuccess,
				}
				ctx = context.WithValue(ctx, contentCtxKey, partials.ShowNotification(msg))
				// Trigger state updates.
				htmxResp := htmx.NewResponse()
				htmxResp = htmxResp.AddTrigger(htmx.Trigger("UpdateState"))
				ctx = context.WithValue(ctx, htmxRespCtxKey, htmxResp)
			case models.UserConfirmationCancel:
				slogctx.FromCtx(ctx).Debug("Subscription removal cancelled.",
					slog.String("subscription_id", subscriptionID),
				)
				// Don't swap any main content for user cancellation.
				// Display a notification acknowledging cancellation of request.
				msg := &models.UserMessage{
					Summary: "Request cancelled.",
					Status:  models.UserMessageStatusInfo,
				}
				ctx = context.WithValue(ctx, contentCtxKey, partials.ShowNotification(msg))
			default:
				slogctx.FromCtx(ctx).Debug("Confirming subscription removal.",
					slog.String("subscription_id", subscriptionID),
				)
				modal := partials.AskQuestion("Unsubscribe?", templ.Attributes{
					"hx-delete": "/subscription/remove/" + subscriptionID,
					"hx-target": "#" + subscriptionID,
					"hx-swap":   "morph:outerHTML",
				})
				ctx = context.WithValue(ctx, contentCtxKey, modal)
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// ShowSubscriptionEditModal retrieves the subscription with the given ID and presents a form for the user to edit it.
func ShowSubscriptionEditModal(api FeedsAPI, subID models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Retrieve user object.
			user, found := models.UserFromCtx(req.Context())
			if !found {
				ProcessResponse(res, req, models.RespInvalidUser())
				return
			}
			// Retrieve subscription.
			subscription := user.GetSubscriptions().FindByID(subID)
			if subscription == nil {
				ProcessResponse(res, req, &models.Response{
					StatusCode: http.StatusNoContent,
					UserMessage: &models.UserMessage{
						Status:  models.UserMessageStatusWarning,
						Summary: "No subscription with matching ID.",
					},
				})
				return
			}
			feed, err := api.GetFeed(req.Context(), subscription.GetFeedID())
			if err != nil {
				ProcessResponse(res, req, models.RespTemporaryIssue("The backend encountered an issue. Please retry.", err))
				return
			}
			subscription.Feed = feed

			// Encapsulate subscription in edit request.
			editRequest := &views.SubscriptionEditRequest{
				Subscription: subscription,
			}
			// Add top categories across items in subscription.
			categories, resp := getItemTopCategories(req.Context(), api, subscription.GetFeedID())
			if !resp.IsError() {
				editRequest.TopCategories = categories
			}
			ctx := context.WithValue(req.Context(), contentCtxKey, editRequest.EditSubscriptionModal())
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SaveSubscription handles saving any user edits to an existing subscription.
func SaveSubscription(api UserAPI, subID models.SubscriptionID, edits *models.SubscriptionCustomisation) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var resp *models.Response
			// Add a new HTMX response writer to the context.
			ctx := req.Context()
			// Retrieve user object.
			user, found := models.UserFromCtx(ctx)
			if !found {
				resp = models.RespInvalidUser()
			}
			if resp != nil && !resp.IsError() {
				// Perform subscription edits.
				user.EditSubscription(subID, edits)
				// Save edits to user object.
				resp = api.UpdateUser(ctx, user.GetID(), map[string]any{
					"subscriptions": user.Subscriptions,
					"updated_at":    time.Now().UTC(),
				})
			}
			if resp != nil && resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			msg := &models.UserMessage{
				Status:  models.UserMessageStatusSuccess,
				Summary: "Subscription updated.",
			}
			// Display a notification acknowledging save.
			ctx = context.WithValue(req.Context(), contentCtxKey, partials.ShowNotification(msg))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// MarkSubscriptions handles marking subscriptions with the given IDs with the given mark.
func MarkSubscriptions(api DataAPI, mark models.Mark, subscriptions ...models.SubscriptionID) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if resp := api.MarkSubscriptions(req.Context(), mark, subscriptions...); resp.IsError() {
			ProcessResponse(res, req, resp)
			return
		}
		res.WriteHeader(http.StatusOK)
	})
}

func UpdateDrawer(api FeedsAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			subscriptions, resp := getAllSubscriptions(req.Context(), api)
			if resp.IsError() {
				slogctx.FromCtx(req.Context()).Warn("Failed to get subscriptions.", slog.Any("error", resp.InternalError))
			} else {
				ctx = context.WithValue(ctx, drawerCtxKey, views.SubscriptionList(subscriptions))
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
