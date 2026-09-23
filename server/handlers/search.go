// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/go-base/pkg/htmx"
	"github.com/immanent-tech/go-base/server/forms"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

const (
	defaultSubscriptionSuggestionsCount = 3
	defaultArticleSuggestionsCount      = 10
)

type SearchSuggestions struct {
	template templ.Component
}

func (h *SearchSuggestions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template).ServeHTTP(res, req)
}

// HandleSearchSuggestions performs a search with the user input and presents suggestions back to the user.
func (m *Manager) HandleSearchSuggestions(svc SubscriptionsService, itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Decode search.
		search, err := forms.DecodeForm[*models.SearchRequest](req)
		if err != nil {
			slogctx.FromCtx(req.Context()).Debug("Get search suggestions failed.",
				slog.Any("error", err))
			res.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		// Ignore empty text string.
		if search.Text == "" {
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Retrieve the user object.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			res.WriteHeader(http.StatusInternalServerError)
			return
		}

		searchJobs, jobCtx := errgroup.WithContext(req.Context())
		defer jobCtx.Done()

		var subscriptions models.Subscriptions
		var articles models.Articles

		// Generate subscription suggestions.
		searchJobs.Go(func() error {
			subscriptions, err = svc.GetSubscriptionSuggestions(
				jobCtx,
				search.Text,
				defaultSubscriptionSuggestionsCount,
				nil,
			)
			if err != nil {
				slogctx.FromCtx(jobCtx).Debug("Get search suggestions: unable to get subscription suggestions.",
					slog.Any("error", err))
			}
			return nil
		})

		// Generate article suggestions.
		searchJobs.Go(func() error {
			items, err := itemSvc.SuggestItems(jobCtx, search)
			if err != nil {
				return fmt.Errorf("search articles: %w", err)
			}
			if len(items) > 0 {
				articles, err = service.GenerateArticles(jobCtx, items)
				if err != nil {
					return fmt.Errorf("generate articles: %w", err)
				}
			}
			return nil
		})

		if err := searchJobs.Wait(); err != nil {
			slogctx.FromCtx(req.Context()).Warn("Get search suggestions: run background jobs failed.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
		}

		// Generate search suggestions object.
		RenderPartial(&SearchSuggestions{
			template: templates.SearchSuggestions(&models.SearchResults{
				Search:        *search,
				Subscriptions: subscriptions,
				Articles:      articles,
			}),
		}).ServeHTTP(res, req)
	}
}

type SearchResults struct {
	title   templates.PageTitle
	results *models.SearchResults
}

func (h *SearchResults) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(templates.SearchResults(h.results),
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

func (h *SearchResults) PartialResponse(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/search":
		res.Header().Add(htmx.HeaderPushURL, "/search?"+h.results.Search.Encode())
		template := templates.SearchResults(h.results)
		if len(h.results.Articles) > 0 {
			// Also update the search filters element.
			template = templ.Join(
				template,
				templates.AdvancedSearch(&h.results.Search, templ.Attributes{"hx-swap-oob": "true"}),
			)
		}
		templ.Handler(template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
		templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
		templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
		templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	case "/search/paginate":
		if len(h.results.Articles) == 0 {
			res.WriteHeader(http.StatusNoContent)
		} else {
			templ.Handler(templates.SearchResults(h.results), templ.WithFragments(templates.PaginateFragment)).
				ServeHTTP(res, req)
			RenderPartial(&PartialTemplate{
				template: templates.SearchPaginationControl(
					&h.results.Search,
					element.WithHXSwapOOB("true"),
				),
			}).ServeHTTP(res, req)
		}
	}
}

// HandleSearchResults performs a search with the user input and renders a page with the search results.
func (m *Manager) HandleSearchResults(itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Retrieve search params.
		search := SearchParamsFromCtx(req.Context())
		if txt := req.FormValue("advanced-search-text"); txt != "" {
			search.Text = txt
		}

		// Retrieve the user object.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Debug("Get user data failed.",
				slog.Any("error", models.ErrCtxValueNotFound))
			http.Redirect(res, req, "/login", http.StatusSeeOther)
			return
		}

		var (
			articles   models.Articles
			categories []models.Category
			pagination models.Pagination
		)

		// Set up background search jobs.
		searchJobs, jobCtx := errgroup.WithContext(req.Context())
		defer jobCtx.Done()

		// Search for results.
		searchJobs.Go(func() error {
			var (
				items models.Items
				err   error
			)
			items, pagination, err = itemSvc.RetrieveItems(jobCtx, search)
			if err != nil {
				return fmt.Errorf("search articles: %w", err)
			}
			if len(items) > 0 {
				articles, err = service.GenerateArticles(jobCtx, items)
				if err != nil {
					return fmt.Errorf("generate articles: %w", err)
				}
			}
			return nil
		})

		// Generate top categories for articles.
		searchJobs.Go(func() error {
			var err error
			categories, err = itemSvc.GetTopItemCategoriesForSearchResults(jobCtx, search)
			if err != nil {
				return fmt.Errorf("get top categories: %w", err)
			}

			return nil
		})

		// Run background requests in parallel and wait for results.
		if err := searchJobs.Wait(); err != nil {
			HandleInternalError(http.StatusInternalServerError, fmt.Errorf("search items: %w", err)).ServeHTTP(res, req)
			return
		}

		// If this search is a search subscription, get the subscription details to add to the results.
		var subscription *models.Subscription
		if search.SubscriptionID != nil {
			allSubscriptions := models.SubscriptionsFromCtx(req.Context())
			if allSubscriptions == nil {
				slogctx.Warn(req.Context(), "Unable to retrieve search subscription details.",
					slog.Any("error", models.ErrCtxValueNotFound))
			} else {
				subscription = allSubscriptions.GetByID(*search.SubscriptionID)
			}
		}

		// Create results object.
		results := &models.SearchResults{
			Search:       *search,
			Subscription: subscription,
			Articles:     articles,
			Categories:   categories,
		}
		results.Search.From = pagination.From

		// Render.
		RenderInternalPage(&SearchResults{
			title: templates.PageTitle{
				Summary:     "Search Results",
				Description: search.Text,
			},
			results: results,
		}).ServeHTTP(res, req)
	}
}

// HandleSearchUpdates handles checking for any new results for the search request and notifying the user.
func (m *Manager) HandleSearchUpdates(itemSvc ItemService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		// Extract the search search.
		search := SearchParamsFromCtx(req.Context())

		// Build query.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get user data.",
				slog.Any("error", models.ErrCtxValueNotFound),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// Count items matching.
		updateCount, err := itemSvc.CountSearchResults(req.Context(), search)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Failed to get updates.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}

		// If updates found, render a notification.
		if updateCount > 0 {
			RenderPartial(&PartialTemplate{template: partials.UpdatesToast(
				element.WithHXMethod(http.MethodGet, "/search"),
				element.WithHXTarget(templates.ContentID.Target()),
				element.WithHXSwap("morph:innerHTML scroll:top transition:true"),
				element.WithHXValues(search),
			)}).ServeHTTP(res, req)
		} else {
			res.WriteHeader(http.StatusNoContent)
		}
	}
}

// AddSubscriptionFilter handles adding a subscription as a search filter.
func (m *Manager) AddSubscriptionFilter() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		subscription, err := parseForm[*models.AddSubscriptionSearchFilterRequest](req)
		if err != nil {
			HandleInternalError(http.StatusUnprocessableEntity, err).ServeHTTP(res, req)
			return
		}

		RenderPartial(&PartialTemplate{
			template: templates.AddSearchSubscriptionFilter(subscription),
		}).ServeHTTP(res, req)
	}
}

// GetSubscriptionFilterSuggestions handles showing a list of subscriptions as suggestions when building a search query.
func (m *Manager) GetSubscriptionFilterSuggestions(svc SubscriptionsService) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		defaultSuggestionCount := 10
		suggestion, err := forms.DecodeForm[*models.GetSubscriptionsSuggestionRequest](req)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Invalid subscription suggestion input.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		subscriptions, err := svc.GetSubscriptionSuggestions(
			req.Context(),
			suggestion.Text,
			defaultSuggestionCount,
			nil,
		)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			slogctx.FromCtx(req.Context()).Error("Unable to get subscription suggestions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		if errors.Is(err, models.ErrNotFound) {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		RenderPartial(&PartialTemplate{
			template: templates.SearchSubscriptionFilterSuggestions(subscriptions),
		}).ServeHTTP(res, req)
	}
}
