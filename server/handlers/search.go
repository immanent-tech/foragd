// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// GetSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func GetSearchSuggestions(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Debug("Get search suggestions failed.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		// Ignore empty text string.
		if request.Text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		// Get user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			slogctx.FromCtx(req.Context()).Debug("Get search suggestions failed.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusForbidden)
			return
		}

		fetchJobs, jobCtx := errgroup.WithContext(req.Context())

		var subscriptions models.Subscriptions
		var articles models.Articles
		sort := models.SortMostRelevant

		fetchJobs.Go(func() error {
			// Generate subscription suggestions.
			subscriptionsQuery := query.Bool(
				query.Filter(
					query.Term("user_id", user.GetID()),
				),
				query.Must(
					query.SearchAsYouType(request.Text, "customisation.nickname"),
				),
			)
			subscriptions, _, err = api.SearchSubscriptions(jobCtx, subscriptionsQuery, 3, &sort, nil)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get subscription suggestions.",
					slog.Any("error", err))
			}
			if len(subscriptions) > 0 {
				err := models.AddSubscriptionDynamicInfo(jobCtx, api, subscriptions)
				if err != nil {
					slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get subscription dynamic data.",
						slog.Any("error", err))
					subscriptions = nil
				}
			}
			// slog.Info("subscriptions done")
			return nil
		})

		fetchJobs.Go(func() error {
			allSubscriptions, err := api.GetAllSubscriptions(jobCtx, query.MatchAll())
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get all subscriptions.",
					slog.Any("error", err))
			}
			// Generate article suggestions.
			itemsQuery := query.Bool(
				query.Filter(
					query.Terms("feed_id", allSubscriptions.GetFeedIDs()...),
				),
				query.Must(
					query.Bool(
						query.Should(
							query.SearchAsYouType(request.Text, "title"),
							query.SearchAsYouType(request.Text, "description"),
							query.SearchAsYouType(request.Text, "content"),
							// Search in categories.
							query.Term(request.Text, "categories"),
							// Search in authors, contributors.
							query.Term(request.Text, "authors"),
							query.Term(request.Text, "contributors"),
						),
					),
				),
			)
			itemResults, _, err := api.SearchItems(jobCtx, itemsQuery, 10, &sort, nil)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get article suggestions.",
					slog.Any("error", err))
			}
			if len(itemResults) > 0 {
				articles, err = models.GenerateArticles(jobCtx, api, itemResults)
				if err != nil {
					slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get article suggestions.",
						slog.Any("error", err))
				}
			}
			return nil
		})

		err = fetchJobs.Wait()
		if err != nil {
			slogctx.FromCtx(req.Context()).Warn("Get search suggestions: run background jobs failed.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
		}

		if len(subscriptions) == 0 && len(articles) == 0 {
			res.WriteHeader(http.StatusNoContent)
		} else {
			renderPartial(templates.SearchSuggestions(request, subscriptions, articles)).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

// GetSearchResults performs a search with the user input and renders a page with the search results.
func GetSearchResults(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		pageTitle := templates.GeneratePageTitle("Search Results")

		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			renderPage(templates.NotFound(), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		// Extract the current pagination value.
		pagination := req.FormValue(models.ParamPagination)
		// Extract any subscription ID for the search and add to the request object.
		subscriptionID := req.FormValue(models.ParamSubscriptionID)
		if subscriptionID != "" {
			request.ID = subscriptionID
		}

		// Embed the request in the context.
		ctx := models.SearchRequestToCtx(req.Context(), *request)
		// If the search request has subscription filters, get subscription details.
		if len(request.Subscriptions) > 0 {
			subscriptions, err := models.GetSubscriptions(req.Context(), api, request.Subscriptions...)
			if err != nil {
				msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
				renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable to retrieve subscriptions: %w", err),
					http.StatusInternalServerError,
				)
			}
			err = models.AddSubscriptionDynamicInfo(req.Context(), api, subscriptions)
			if err != nil {
				msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
				renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable to retrieve subscriptions: %w", err),
					http.StatusInternalServerError,
				)
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
		}

		// Retrieve the user object.
		user, err := models.UserFromCtx(ctx)
		if err != nil {
			msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
			renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve subscriptions: %w", err),
				http.StatusInternalServerError,
			)
		}

		// Find articles that match search request.
		var articles models.Articles
		query, err := models.BuildSearchResultsQuery(ctx, api, user, request)
		if err != nil {
			msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
			renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve subscriptions: %w", err),
				http.StatusInternalServerError,
			)
		}
		itemResults, pagination, err := api.SearchItems(ctx, query, 15, &request.Sort, &pagination)
		if err != nil {
			msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
			renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve subscriptions: %w", err),
				http.StatusInternalServerError,
			)
		}
		if len(itemResults) > 0 {
			articles, err = models.GenerateArticles(ctx, api, itemResults)
			if err != nil {
				msg := models.NewErrorMessage("Unable to process request", "This might be a temporary issue, please try again.")
				renderPage(templates.ErrorPage(msg), pageTitle).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable to retrieve subscriptions: %w", err),
					http.StatusInternalServerError,
				)
			}
		}

		if strings.HasSuffix(chi.RouteContext(ctx).RoutePattern(), "/paginate") { //nolint:nestif
			if len(articles) > 0 {
				renderPartial(templates.SearchResults(articles, pagination)).ServeHTTP(res, req.WithContext(ctx))
			} else {
				res.WriteHeader(http.StatusNoContent)
			}
		} else {
			var template templ.Component
			if len(articles) > 0 {
				// Pagination request, just display next set of results.
				template = templates.SearchResultsGrid(request, articles, pagination)
			} else {
				template = templates.NoSearchResults()
			}
			if IsHTMX(req) {
				template = templ.Join(template, templates.SearchFilters(templ.Attributes{"hx-swap-oob": "true"}))
			}
			res.Header().Add(htmx.HeaderPushURL, "/search?"+request.Query())
			renderPage(template, templates.GeneratePageTitle("Search Results")).ServeHTTP(res, req.WithContext(ctx))
		}

		return nil
	})).ServeHTTP
}

// WatchSearchResults handles watching the search results for any updates and rendering a notification to the user to refresh the page.
func WatchSearchResults(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get user data.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to get user data: %w", err)
		}
		// Extract the search request.
		request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil || !valid {
			return fmt.Errorf("unable to get search request updates: %w", err)
		}
		// Build query.
		query, err := models.BuildSearchResultsQuery(req.Context(), api, user, request)
		if err != nil {
			return fmt.Errorf("unable to get search request updates: %w", err)
		}
		// Watch for updates to search results.
		watchForUpdates(api, query).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddSubscriptionFilter handles adding a subscription as a search filter.
func AddSubscriptionFilter() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := req.FormValue("subscription_id")
		name := req.FormValue("subscription_name")
		input := req.FormValue("subscriptions-input-name")
		if id == "" || name == "" || input == "" {
			res.WriteHeader(http.StatusUnprocessableEntity)
			return nil
		}
		renderPartial(templates.AddSearchSubscriptionFilter(id, name, input)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// GetSubscriptionFilterSuggestions handles showing a list of subscriptions as suggestions when building a search query.
func GetSubscriptionFilterSuggestions(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		text := req.FormValue("subscription-text")
		if text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		subscriptions, err := models.GetSubscriptionSuggestions(req.Context(), api, text)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		if errors.Is(err, models.ErrNotFound) {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		renderPartial(templates.SearchSubscriptionFilterSuggestions(subscriptions)).ServeHTTP(res, req)
	}).ServeHTTP
}
