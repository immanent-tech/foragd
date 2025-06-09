// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/layouts/settings"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
)

// GenerateArticleCollection handles searching for articles with the current filters and then generating cards for each found article.
func GenerateArticleCollection(api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			articles, pagination, resp := searchArticles(req.Context(), api, pagination, subIDs...)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}

			cards := views.GenerateArticleCards(req.Context(), articles, pagination)
			if len(cards) > 0 {
				cardLayout := partials.CardGrid(cards...)
				cardControls := partials.CardControls(
					views.RefreshAction(),
					views.UpdateSorting(models.CollectionArticles),
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

// PaginateArticleCollection handles fetching the next set of articles and creating cards from them.
func PaginateArticleCollection(api FeedsAPI, pagination models.Pagination, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			articles, pagination, resp := searchArticles(req.Context(), api, pagination, subIDs...)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			cards := views.GenerateArticleCards(req.Context(), articles, pagination)
			ctx := context.WithValue(req.Context(), contentCtxKey, templ.Join(cards...))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateArticle handles displaying an item as an article.
func GenerateArticle(api FeedsAPI, itemID models.ItemID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			articles, resp := getArticles(req.Context(), api, itemID)
			if resp.IsError() || len(articles) == 0 {
				ProcessResponse(res, req, resp)
				return
			}
			articleLayout := views.BuildArticleLayout(articles[0])

			ctx := req.Context()
			ctx = context.WithValue(ctx, contentCtxKey, articleLayout)
			ctx = context.WithValue(ctx, titleCtxKey, articles[0].Item.GetTitle())
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateSubscriptionCollection handles searching for subscriptions with the current filters and then generating cards for each found subscription.
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
					views.UpdateSorting(models.CollectionSubscriptions),
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

// PaginateSubscriptionCollection handles fetching the next set of subscriptions and creating cards from them.
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

// NewSubscription generates a form for the user to enter details to add a new subscription.
func NewSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), contentCtxKey, views.NewSubscriptionModal(&models.SubscriptionRequest{}))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ParseNewSubscriptionRequest will extract the subscription request, validate it and then store it in the context for
// further processing.
func ParseNewSubscriptionRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
		if err != nil || !valid {
			details := err.Error()
			request.Result = &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Unable to parse request.",
				Details: &details,
			}
			ctx = context.WithValue(ctx, contentCtxKey, views.NewSubscriptionModal(request))
		} else {
			ctx = context.WithValue(ctx, subscriptionRequestsCtxKey, models.SubscriptionRequests{request})
		}

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// NewSubscriptionRequestResult handles processing the result of an add subscription request.
func NewSubscriptionRequestResult(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract the processed request from the context.
		requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			next.ServeHTTP(res, req)
			return
		}
		// Display the modal with the request results shown.
		ctx := context.WithValue(req.Context(), contentCtxKey, views.NewSubscriptionModal(requests[0]))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// NewSubscriptionsImport handles setting up a new subscription import process for the user.
func NewSubscriptionsImport(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), contentCtxKey, views.ImportSubscriptionLayout())
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
					ctx = context.WithValue(ctx, contentCtxKey, views.ImportFromOPML())
				}
			case http.MethodPost:
				switch importMethod {
				case "opml_file":
					// Decode the OPML file form input.
					opmlFile := &models.OPMLFile{}
					opmlFile, valid, err := forms.DecodeMultipartFile(req, "data", opmlFile)
					if err != nil || !valid {
						ProcessResponse(res, req, &models.Response{
							StatusCode: http.StatusNoContent,
							UserMessage: &models.UserMessage{
								Status:  models.UserMessageStatusWarning,
								Summary: "Could not parse OPML file.",
							},
							InternalError: err,
						})
						return
					}
					opmlImport, err := opmlFile.Parse()
					if err != nil {
						ProcessResponse(res, req, &models.Response{
							StatusCode: http.StatusNoContent,
							UserMessage: &models.UserMessage{
								Status:  models.UserMessageStatusWarning,
								Summary: "Could not parse OPML file.",
							},
							InternalError: err,
						})
						return
					}
					// Extract the individual feeds from the OPML object and create a subscription
					// request for each one.
					feeds := opmlImport.ExtractRSS()
					requests := make(models.SubscriptionRequests, 0, len(feeds))
					for _, feed := range feeds {
						requests = append(requests, &models.SubscriptionRequest{URL: feed.XMLURL})
					}
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
		// Get the request.
		requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
		if !found {
			ProcessResponse(res, req, &models.Response{
				StatusCode: http.StatusNoContent,
				UserMessage: &models.UserMessage{
					Status:  models.UserMessageStatusWarning,
					Summary: "No response!",
				},
			})
		}
		ctx := context.WithValue(req.Context(), contentCtxKey, views.ImportResults(requests))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// ProcessSubscriptionRequests handles adding new subscription via either the add or import user functionality. It
// handles: matching and filtering out requests against existing subscriptions, matching requests to existing feeds,
// creating new feeds as necessary and finally creating user subscriptions.
//
//nolint:gocognit,gocyclo,funlen // breaking up this function would actually add debugging/development complexity.
func ProcessSubscriptionRequests(api BackendAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			generateErrorResponse := func(ctx context.Context, msg *models.UserMessage) context.Context {
				htmxResp := htmx.NewResponse().Reswap(htmx.SwapOuterHTML).Retarget(partials.ModalID.Target())
				ctx = context.WithValue(ctx, htmxRespCtxKey, htmxResp)
				ctx = context.WithValue(ctx, contentCtxKey, msg)
				return ctx
			}

			user, found := models.UserFromCtx(req.Context())
			if !found {
				ctx := generateErrorResponse(req.Context(), models.RespInvalidUser().UserMessage)
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}
			requests, found := req.Context().Value(subscriptionRequestsCtxKey).(models.SubscriptionRequests)
			if !found {
				next.ServeHTTP(res, req)
				return
			}

			// STEP 1: Match requests to existing feeds.
			slogctx.FromCtx(req.Context()).Debug("Matching subscription requests to existing feeds...")
			var (
				feedPagination *models.Pagination
				existingFeeds  models.Feeds
			)
			for {
				count := 100
				feeds, nextResults, err := api.SearchFeeds(req.Context(), query.URLs("source", requests.URLs()...), count, nil, feedPagination)
				if err != nil {
					ctx := generateErrorResponse(req.Context(), models.RespServerError("Backend error occurred.", err).UserMessage)
					next.ServeHTTP(res, req.WithContext(ctx))
					return
				}

				existingFeeds = append(existingFeeds, feeds...)

				if len(feeds) < count {
					break
				}
				feedPagination = &nextResults
			}

			slogctx.FromCtx(req.Context()).Debug("Retrieved existing feeds.",
				slog.Int("count", len(existingFeeds)),
			)

			// Loop over existing feeds.
			for request := range slices.Values(requests) { //nolint:contextcheck
				slogctx.FromCtx(req.Context()).Debug("Searching for existing feed.",
					slog.String("url", request.GetURL()),
				)
				feed := existingFeeds.FindByURL(request.GetURL())
				if feed == nil {
					slogctx.FromCtx(req.Context()).Debug("No match for request url.", slog.String("url", request.GetURL()))
					// No existing feed matches request, will add new feed in next step.
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
					slog.String("subscription_id", request.Subscription.GetID()),
					slog.String("subscription_nickname", request.UserNickname),
					slog.String("feed_id", feed.GetID()),
					slog.String("feed_name", feed.GetTitle()),
					slog.String("source_url", request.GetURL()),
				)
			}

			// STEP 2: Create new feeds where needed.
			slogctx.FromCtx(req.Context()).Debug("Creating new feeds as needed...")
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
					slog.String("subscription_id", request.Subscription.GetID()),
					slog.String("subscription_nickname", request.UserNickname),
					slog.String("feed_id", newFeed.GetID()),
					slog.String("feed_name", newFeed.GetTitle()),
					slog.String("source_url", request.GetURL()),
				)
			}

			// // STEP 3: Add new feeds.
			// slogctx.FromCtx(req.Context()).Debug("Adding new feeds for subscription requests.")
			// // Add the new feeds.
			// newFeedsResp, err := api.AddFeeds(req.Context(), newFeeds...)
			// if err != nil {
			// 	ctx := generateErrorResponse(req.Context(), models.RespServerError("Backend error occurred.", err).UserMessage)
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
			// 	// Get the request that matches the new feed.
			// 	request := requests.FindByURL(feed.URL)
			// 	if request == nil {
			// 		slogctx.FromCtx(req.Context()).Warn("Result with unmatched request", slog.Any("result", result))
			// 		continue
			// 	}
			// 	// If the new feed failed to be added, record an error against the request.
			// 	if _, err := result.State(); err != nil {
			// 		details := err.Error()
			// 		request.Result = &models.UserMessage{
			// 			Status:  models.UserMessageStatusError,
			// 			Summary: "Add subscription failed.",
			// 			Details: &details,
			// 		}
			// 	}
			// }

			// STEP 4: Add new subscriptions.
			slogctx.FromCtx(req.Context()).Debug("Adding subscriptions.")
			// Validate subscriptions.
			validRequests := requests.FilterValid().FilterWithSubscription()
			// Add valid subscriptions.
			// if resp := addSubscriptions(req.Context(), api, validRequests.Subscriptions()); resp.IsError() {
			// 	ctx := generateErrorResponse(req.Context(), resp.UserMessage)
			// 	next.ServeHTTP(res, req.WithContext(ctx))
			// 	return
			// } else {
			for request := range slices.Values(validRequests) {
				request.Result = &models.UserMessage{
					Status:  models.UserMessageStatusSuccess,
					Summary: fmt.Sprintf("Subscription %s created!", request.String()),
				}
			}
			// }

			// Store the processed requests in the context for the next handler.
			ctx := context.WithValue(req.Context(), subscriptionRequestsCtxKey, requests)
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
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

// EditSubscription retrieves the subscription with the given ID and presents a form for the user to edit it.
func EditSubscription(api FeedsAPI, subID models.SubscriptionID) func(next http.Handler) http.Handler {
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
			feeds, err := api.GetFeeds(req.Context(), subscription.GetFeedID())
			if err != nil {
				ProcessResponse(res, req, models.RespTemporaryIssue("The backend encountered an issue. Please retry.", err))
				return
			}
			subscription.Feed = feeds[0]

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
func MarkSubscriptions(api UserAPI, mark models.Mark, subscriptions ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if resp := markSubscriptions(req.Context(), api, mark, subscriptions...); resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}

// MarkArticles handles marking articles with the given mark.
func MarkArticles(api BackendAPI, mark models.Mark, items ...models.ItemID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Mark the feeds.
			if resp := markArticles(req.Context(), api, mark, items...); resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			next.ServeHTTP(res, req)
		})
	}
}

// GenerateSettings handles displaying the user settings page.
func GenerateSettings(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		settingsLayout := settings.SettingsContent()
		ctx := req.Context()
		ctx = context.WithValue(ctx, contentCtxKey, settingsLayout)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// GenerateDrawerContent handles generating updated content for the drawer.
func GenerateDrawerContent(api FeedsAPI) func(next http.Handler) http.Handler {
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
