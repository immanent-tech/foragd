// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"context"
	"log/slog"
	"maps"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/providers/elastic"
	"github.com/joshuar/go-feed-me/providers/elastic/bulk"
	"github.com/joshuar/go-feed-me/providers/elastic/query"
	"github.com/joshuar/go-feed-me/providers/elastic/schema"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/web/templates/layouts/settings"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/views"
)

// GenerateArticleCollection handles searching for articles with the current filters and then generating cards for each found article.
func GenerateArticleCollection(api FeedsAPI, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			filters := models.FiltersFromCtx(ctx)
			articles, pagination, resp := filterArticles(req.Context(), api, &filters)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}

			cards := views.GenerateArticleCards(req.Context(), articles)
			if len(cards) > 0 {
				// Add pagination htmx props to last article.
				if len(cards) == filters.CountAsInt() {
					cards = append(cards, views.PaginationControl(ctx, pagination))
				}
				cardLayout := partials.CardGrid(cards...)
				cardControls := partials.CardControls(
					views.RefreshAction(),
					views.UpdateSorting(models.CollectionArticles),
					views.UpdateFilters(articles.GetCategoryCounts()),
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
func PaginateArticleCollection(api FeedsAPI, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters := models.FiltersFromCtx(req.Context())
			articles, pagination, resp := filterArticles(req.Context(), api, &filters)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			cards := views.GenerateArticleCards(req.Context(), articles)
			// Add pagination htmx props to last article.
			if len(cards) == filters.CountAsInt() {
				cards = append(cards, views.PaginationControl(req.Context(), pagination))
			}

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
			ctx = context.WithValue(ctx, titleCtxKey, articles[0].GetTitle())
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateSubscriptionCollection handles searching for subscriptions with the current filters and then generating cards for each found subscription.
func GenerateSubscriptionCollection(api FeedsAPI, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters := models.FiltersFromCtx(req.Context())
			subscriptions, pagination, resp := filterSubscriptions(req.Context(), api, &filters)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			ctx := req.Context()
			cards := views.GenerateSubscriptionCards(req.Context(), subscriptions)
			if len(cards) > 0 {
				// Add pagination htmx props to last article.
				if len(cards) == filters.CountAsInt() {
					cards = append(cards, views.PaginationControl(ctx, pagination))
				}

				cardLayout := partials.CardGrid(cards...)
				cardControls := partials.CardControls(
					views.RefreshAction(),
					views.UpdateSorting(models.CollectionSubscriptions),
					views.UpdateFilters(models.GetCategoryCounts(slices.Values(subscriptions))),
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
func PaginateSubscriptionCollection(api FeedsAPI, subIDs ...models.SubscriptionID) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			filters := models.FiltersFromCtx(req.Context())
			subscriptions, pagination, resp := filterSubscriptions(req.Context(), api, &filters)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			cards := views.GenerateSubscriptionCards(req.Context(), subscriptions)
			// Add pagination htmx props to last article.
			if len(cards) == filters.CountAsInt() {
				cards = append(cards, views.PaginationControl(req.Context(), pagination))
			}

			ctx := context.WithValue(req.Context(), contentCtxKey, templ.Join(cards...))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// NewSubscription generates a form for the user to enter details to add a new subscription.
func NewSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), contentCtxKey, views.NewSubscriptionModal(&models.SubscriptionRequest{}, nil))
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
			result := &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Unable to parse request.",
				Details: &details,
			}
			ctx = context.WithValue(ctx, contentCtxKey, views.NewSubscriptionModal(request, result))
		} else {
			ctx = context.WithValue(ctx, subscriptionRequestsCtxKey, request)
		}

		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// NewSubscriptionRequestResult handles processing the result of an add subscription request.
func NewSubscriptionRequestResult(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		// Extract the processed request from the context.
		results, found := req.Context().Value(addSubscriptionsResultsCtxKey).([]*models.UserMessage)
		if !found {
			next.ServeHTTP(res, req)
			return
		}
		// Display the modal with the request results shown.
		ctx := context.WithValue(req.Context(), contentCtxKey, views.NewSubscriptionModal(&models.SubscriptionRequest{}, results[0]))
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
					requests := make([]*models.SubscriptionRequest, 0, len(feeds))
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
		results, found := req.Context().Value(addSubscriptionsResultsCtxKey).([]*models.UserMessage)
		if !found {
			ProcessResponse(res, req, &models.Response{
				StatusCode: http.StatusNoContent,
				UserMessage: &models.UserMessage{
					Status:  models.UserMessageStatusWarning,
					Summary: "No response!",
				},
			})
		}
		ctx := context.WithValue(req.Context(), contentCtxKey, views.ImportResults(results...))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

// AddSubscriptions handles adding new subscription via either the add or import user functionality. It
// handles: matching and filtering out requests against existing subscriptions, matching requests to existing feeds,
// creating new feeds as necessary and finally creating user subscriptions.
//
//nolint:gocognit,gocyclo,funlen // breaking up this function would actually add debugging/development complexity.
func AddSubscriptions(api BackendAPI) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			var results []*models.UserMessage

			newSubscriptionNeeded := make(map[*models.SubscriptionRequest]*models.Feed)
			newFeedsNeeded := make(map[*models.SubscriptionRequest]*models.Feed)

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
			// Extract the requests from the context.
			data := req.Context().Value(subscriptionRequestsCtxKey)
			if data == nil {
				next.ServeHTTP(res, req)
				return
			}
			var requests []*models.SubscriptionRequest
			switch value := data.(type) {
			case *models.SubscriptionRequest:
				requests = append(requests, value)
			case []*models.SubscriptionRequest:
				requests = append(requests, value...)
			default:
				next.ServeHTTP(res, req)
				return
			}

			// STEP 1: Match requests to existing feeds.
			slogctx.FromCtx(req.Context()).Debug("Matching subscription requests to existing feeds...")
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
				feeds, nextResults, err := api.SearchFeeds(req.Context(), query.URLs("source", feedURLs...), count, nil, feedPagination)
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

			// // Loop over existing feeds.
			for request := range slices.Values(requests) { //nolint:contextcheck
				slogctx.FromCtx(req.Context()).Debug("Searching for existing feed.",
					slog.String("url", request.GetURL()),
				)
				feed := existingFeeds.FindByURL(request.GetURL())
				switch {
				case feed == nil: // no existing feed, create a new one.
					newFeed, err := models.NewFeedFromURL(req.Context(), request.GetURL())
					if err != nil {
						details := err.Error()
						results = append(results,
							&models.UserMessage{
								Status:  models.UserMessageStatusError,
								Summary: "Could not add " + request.String(),
								Details: &details,
							})
						continue
					}
					newFeedsNeeded[request] = newFeed
				case user.IsSubscribed(feed.GetID()): // user already subscribed, ignore request.
					details := "A subscription for " + feed.String() + " already exists."
					results = append(results,
						&models.UserMessage{
							Status:  models.UserMessageStatusInfo,
							Summary: "Already Subscribed",
							Details: &details,
						})
				default: // existing feed.
					newSubscriptionNeeded[request] = feed
				}
			}

			// STEP 2: Add new feeds.
			slogctx.FromCtx(req.Context()).Debug("Adding new feeds for subscription requests.")
			// Add the new feeds.
			newFeedsResp, err := api.AddFeeds(req.Context(), slices.Collect(maps.Values(newFeedsNeeded))...)
			if err != nil {
				ctx := generateErrorResponse(req.Context(), models.RespServerError("Backend error occurred.", err).UserMessage)
				next.ServeHTTP(res, req.WithContext(ctx))
				return
			}
			// Process the add feed results.
			for request, feed := range newFeedsNeeded {
				idx := slices.IndexFunc(newFeedsResp.Responses, func(resp *bulk.OperationResponse) bool {
					if resp.Id_ != nil {
						return *resp.Id_ == feed.GetID()
					}
					return false
				})
				if idx != -1 {
					// Add new feed for request has a response.
					if _, err := newFeedsResp.Responses[idx].State(); err != nil {
						details := err.Error()
						results = append(results,
							&models.UserMessage{
								Status:  models.UserMessageStatusError,
								Summary: "Could not add " + request.String(),
								Details: &details,
							})
						continue
					}
					// Success, add request to map of subscription needed.
					newSubscriptionNeeded[request] = feed
				}
			}

			// STEP 3: Add new subscriptions.
			slogctx.FromCtx(req.Context()).Debug("Adding subscriptions.")
			// Generate subscriptions from request and feed data.
			for request := range newSubscriptionNeeded {
				// If the user has added customisations, add those.
				if request.Title != "" || len(request.Categories) > 0 {
					customisation := make(map[string]any)
					if request.Title != "" {
						customisation["user_nickname"] = request.Title
					}
					if len(request.Categories) > 0 {
						customisation["user_categories"] = request.Categories
					}
					if err := api.UpdateSubscriptionCustomisation(req.Context(), request.SubscriptionID, customisation); err != nil {
						details := err.Error()
						results = append(results,
							&models.UserMessage{
								Status:  models.UserMessageStatusError,
								Summary: "Could not add " + request.String(),
								Details: &details,
							})
						continue
					}
				}
				// Create and add a new subscription state to the user object.
				state := models.NewSubscriptionState(request.SubscriptionID, request.FeedID)
				states := user.SubscriptionStates
				states = append(states, *state)
				if err := api.UpdateUser(req.Context(), request.SubscriptionID, map[string]any{
					"subscription_states": states,
				}); err != nil {
					details := err.Error()
					results = append(results,
						&models.UserMessage{
							Status:  models.UserMessageStatusError,
							Summary: "Could not add " + request.String(),
							Details: &details,
						})
					continue
				}
			}

			// Store the results in the context for the next handler.
			ctx := context.WithValue(req.Context(), addSubscriptionsResultsCtxKey, results)
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
			// Retrieve subscription.
			subscriptions, resp := getSubscriptions(req.Context(), api, subID)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}
			subscription := subscriptions[0]

			request := &models.SubscriptionEdit{
				SubscriptionID: subscription.GetID(),
				Title:          subscription.GetTitle(),
				Categories:     subscription.GetCategories(),
			}

			// Add top categories across items in subscription.
			categories, resp := getItemTopCategories(req.Context(), api, subscription.GetFeedID())
			if !resp.IsError() {
				request.TopCategories = categories
			}
			ctx := context.WithValue(req.Context(), contentCtxKey, views.EditSubscriptionModal(request))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// SaveSubscription handles saving any user edits to an existing subscription.
func SaveSubscription(api UserAPI, edits *models.SubscriptionCustomisation) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Add a new HTMX response writer to the context.
			ctx := req.Context()
			if err := api.UpdateSubscriptionCustomisation(ctx, edits.SubscriptionID, map[string]any{
				"user_nickname":   edits.Title,
				"user_categories": edits.Categories,
			}); err != nil {
				ProcessResponse(res, req, models.RespServerError("Failed to update subscription.", err))
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
			subscriptions, resp := getSubscriptions(req.Context(), api)
			if resp.IsError() {
				slogctx.FromCtx(req.Context()).Warn("Failed to get subscriptions.", slog.Any("error", resp.InternalError))
			} else {
				ctx = context.WithValue(ctx, drawerCtxKey, views.SubscriptionList(subscriptions))
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

// GenerateDrawerContent handles generating updated content for the drawer.
func GenerateSearchSuggestions(api FeedsAPI, searchTerms string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			subscriptions, articles, resp := getSearchSuggestions(ctx, api, searchTerms)
			if resp.IsError() {
				ProcessResponse(res, req, resp)
				return
			}

			if len(subscriptions) > 0 || len(articles) > 0 {
				suggestions := make([]templ.Component, 0, len(articles)+1)

				if len(subscriptions) > 0 {
					// Add subscription suggestions.
					suggestions = append(suggestions, views.SearchSuggestionHeader("Subscriptions"))
					for subscription := range slices.Values(subscriptions) {
						suggestions = append(suggestions, views.SearchSuggestionSubscription(subscription))
					}
				}
				if len(articles) > 0 {
					// Add article suggestions.
					suggestions = append(suggestions, views.SearchSuggestionHeader("Articles"))
					for article := range slices.Values(articles) {
						suggestions = append(suggestions, views.SearchSuggestionArticle(article))
					}
				}

				ctx = context.WithValue(ctx, contentCtxKey, views.SearchSuggestions(suggestions...))
			}
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}

func NewUserSignup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), pageCtxKey, views.SignUpPage(models.NewUserSignup()))
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func ProcessUserSignup(userBackendAPI UserBackendAPI, userFrontendAPI UserAPI, signupRequest *models.UserSignupRequest) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			// Create the user in the auth backend.
			userID, err := userBackendAPI.Create(req.Context(), signupRequest)
			if err != nil {
				ProcessResponse(res, req, models.RespServerError("Unable to create a new user account.", err))
				return
			}
			// Create new user in the database backend.
			ctx := elastic.UserIndexToCtx(req.Context(), schema.UsersSchemaPrefix)
			err = userFrontendAPI.AddUser(ctx, userID)
			if err != nil {
				ProcessResponse(res, req, models.RespServerError("Unable to create a new user account.", err))
				return
			}
			signupRequest.Msg = &models.UserMessage{
				Status:  models.UserMessageStatusSuccess,
				Summary: "Account created!",
			}
			ctx = context.WithValue(req.Context(), contentCtxKey, views.SignupForm(signupRequest))
			next.ServeHTTP(res, req.WithContext(ctx))
		})
	}
}
