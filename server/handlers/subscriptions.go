// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/cespare/xxhash/v2"
	"github.com/go-chi/chi/v5"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/sync/errgroup"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListSubscriptions handles fetching subscriptions based on the given page filters and displaying them. When the
// request method is GET (i.e. initial page load), the subscriptions are shown in a grid layout. When the request method
// is POST (i.e. pagination request), the subscriptions are shown as a list.
func ListSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(func(res http.ResponseWriter, req *http.Request) {
			list := func(res http.ResponseWriter, req *http.Request) error {
				// Generate request object.
				request := &models.ListRequest{
					Filters:    *models.PageFiltersFromCtx(req.Context(), req.URL.Path),
					Pagination: req.FormValue(models.ParamPagination),
				}
				if err := request.Valid(); err != nil {
					return &models.APIError{
						InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
						StatusCode:    http.StatusUnprocessableEntity,
					}
				}

				// Redirect to include query parameters in address bar.
				switch {
				case htmx.IsHTMX(req):
					res.Header().Set(htmx.HeaderReplaceUrl, req.URL.Path+"?"+request.Filters.QueryString())
				case len(req.URL.Query()) == 0:
					http.Redirect(res, req, req.URL.Path+"?"+request.Filters.QueryString(), http.StatusSeeOther)
				}
				var (
					subscriptions models.Subscriptions
					counts        models.CategoryCounts
					err           error
					template      templ.Component
				)

				// Remove any subscription filters if this is a history restore request (i.e. back button clicked).
				if htmx.IsHistoryRestoreRequest(req) {
					request.Filters.Subscriptions = nil
				}

				// Get subscriptions matching filters.
				wg, jobCtx := errgroup.WithContext(req.Context())
				defer jobCtx.Done()
				wg.Go(func() error {
					subscriptions, request.Pagination, err = models.FilterSubscriptions(jobCtx, request)
					if err != nil && !errors.Is(err, elastic.ErrNotFound) {
						return fmt.Errorf("filter subscriptions: %w", err)
					}
					return nil
				})
				// Get all subscription categories.
				wg.Go(func() error {
					counts, err = models.GetAllSubscriptionCategories(jobCtx)
					if err != nil {
						slogctx.FromCtx(jobCtx).Warn("Could not get all subscription categories.",
							slog.Any("error", err),
						)
					}
					return nil
				})

				// Wait for fetch jobs to finish.
				if err := wg.Wait(); err != nil {
					return &models.APIError{
						InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
						StatusCode:    http.StatusInternalServerError,
					}
				}

				// Build response object.
				response := &models.ListSubscriptionsResponse{
					CategoryCounts: counts,
					Filters:        request.Filters,
					Pagination:     request.Pagination,
					Subscriptions:  subscriptions,
				}

				// Choose rendering method based on method (get = page, post = partial).
				template = templates.ListSubscriptions(response)
				ctx := templates.PageTitleToCtx(req.Context(), "Subscriptions")
				switch req.Method {
				case http.MethodGet:
					renderPage(wrapContent(req.WithContext(ctx), template)).ServeHTTP(res, req.WithContext(ctx))
				case http.MethodPost:
					renderPartial(template).ServeHTTP(res, req.WithContext(ctx))
				}
				return nil
			}
			switch req.Method {
			case http.MethodGet:
				showOnError(list).ServeHTTP(res, req)
			case http.MethodPost:
				notifyOnError(list).ServeHTTP(res, req)
			}
		}).ServeHTTP
}

func PaginateSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters).
		ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
			// Generate request object.
			request := &models.ListRequest{
				Filters:    *models.PageFiltersFromCtx(req.Context(), req.URL.Path),
				Pagination: req.FormValue(models.ParamPagination),
			}
			if err := validation.Validate.Struct(request); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Could not list more subscriptions",
						"This might be temporary, please try again.",
					),
				}
			}

			// Get subscriptions matching filters.
			var (
				subscriptions models.Subscriptions
				err           error
			)
			subscriptions, request.Pagination, err = models.FilterSubscriptions(req.Context(), request)
			if err != nil && !errors.Is(err, elastic.ErrNotFound) {
				return &models.APIError{
					InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Could not list more subscriptions",
						"This might be temporary, please try again.",
					),
				}
			}

			// Build response object.
			response := &models.ListSubscriptionsResponse{
				Filters:       request.Filters,
				Pagination:    request.Pagination,
				Subscriptions: subscriptions,
			}

			// Render appropriate content.
			if len(subscriptions) > 0 {
				renderPartial(templates.PaginateSubscriptions(response)).ServeHTTP(res, req)
			} else {
				res.WriteHeader(http.StatusNoContent)
				return nil
			}

			return nil
		})).ServeHTTP
}

// MarkSubscription handles marking a subscription as read/unread and updates the UI accordingly.
func MarkSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request values.
		subscriptionID := chi.URLParam(req, models.ParamSubscriptionID)
		request := &models.MarkSubscriptionsRequest{
			Subscriptions: []models.SubscriptionID{subscriptionID},
			Mark:          models.Mark(chi.URLParam(req, models.ParamMark)),
			View:          models.View(req.FormValue(models.ParamView)),
		}
		if err := request.Valid(); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		// Mark subscription.
		if err := models.MarkSubscriptions(req.Context(), request.Mark, request.Subscriptions...); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("mark subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		// Determine the URL the request came from.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			if err := SetRedirect(res, HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
			}); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("set redirect: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to mark subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
		}
		if strings.Contains(currentURL, "/list/articles") {
			// If the current URL is /list/articles, return to /list/subscriptions.
			if err := SetRedirect(res, HXLocationRequest{
				Path:   "/list/subscriptions",
				Target: templates.ContentID.Target(),
				Values: models.PageFiltersFromCtx(req.Context(), "/list/subscriptions").Values(),
			}); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("set redirect: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to mark subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			res.WriteHeader(http.StatusOK)
		} else {
			// Else swap content apprpriately.
			switch request.View {
			case models.ViewRead, models.ViewUnread:
				res.Header().Set(htmx.HeaderReswap, "delete transition:true")
				res.WriteHeader(http.StatusOK)
			case models.ViewAll:
				subscription, err := models.GetSubscription(req.Context(), request.Subscriptions[0], models.GetSubscriptionsDynamicInfo(true))
				if err != nil {
					return &models.APIError{
						InternalError: fmt.Errorf("get subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to mark subscription",
							"This might be a temporary issue, please try again.",
						),
					}
				}
				res.Header().Set(htmx.HeaderReswap, "outerHTML transition:true")
				renderPartial(templates.SubscriptionCard(subscription)).ServeHTTP(res, req)
			}
		}
		return nil
	})).ServeHTTP
}

// MarkSubscriptions handles marking a list of subscriptions.
func MarkSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request parameters.
		request, valid, err := forms.DecodeForm[*models.MarkSubscriptionsRequest](req)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("decode mark subscription request: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		if !valid {
			return &models.APIError{
				InternalError: fmt.Errorf("validate mark subscription request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		// Determine what mark to apply from view and where to redirect.
		switch request.View {
		case models.ViewUnread:
			err = SetRedirect(res, HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
			})
		case models.ViewRead:
			err = SetRedirect(res, HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
			})
		default:
			err = SetRedirect(res, HXLocationRequest{
				Path:   "/list/subscriptions",
				Target: templates.ContentID.Target(),
				Values: models.PageFiltersFromCtx(req.Context(), req.URL.Path).Values(),
			})
		}
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("set redirect: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		// Mark subscriptions.
		err = models.MarkSubscriptions(req.Context(), request.Mark, request.Subscriptions...)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("mark subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}

// RemoveSubscription handles removing (unsubscribing) from a subscription.
func RemoveSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		request := &models.RemoveSubscriptionRequest{
			SubscriptionID: chi.URLParam(req, models.ParamSubscriptionID),
			Nickname:       req.FormValue("nickname"),
		}
		if err := request.Valid(); err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("validate remove subscription request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to remove subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		switch req.FormValue("confirmed") {
		case "false":
			renderPartial(templates.RemoveSubscriptionModal(request)).ServeHTTP(res, req)
		case "true":
			if err := models.RemoveSubscriptions(req.Context(), request.SubscriptionID); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("remove subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to remove subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			// Show success notification.
			renderPartial(
				templates.Notification(
					models.NewSuccessMessage("Unsubscribed from "+request.Nickname, ""),
					templates.DefaultNotificationTimeout,
				),
			).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// EditSubscription handles presenting the user with a form for editing a subscription.
func EditSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Get the subscription.
		subscription, err := models.GetSubscription(req.Context(), id)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to edit subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		var template templ.Component
		var pageTitle string
		ctx := req.Context()
		switch subscription.GetSubscriptionType() {
		case models.SubscriptionTypeFeed:
			// Convert metadata into edit request data.
			request := &models.EditSubscriptionRequest{
				SubscriptionID:         id,
				Nickname:               subscription.GetTitle(),
				Categories:             subscription.Customisation.Categories,
				ImageURL:               subscription.GetImage(),
				ShowFullArticleContent: subscription.Settings.ShowFullArticleContent,
				ArticleFilters:         subscription.FeedData.ArticleFilters,
			}
			// Get top categories across items in subscription feed and add as suggested categories for the
			// subscription.
			if categories, resp := models.GetArticleTopCategories(ctx, subscription.FeedData.GetFeedID()); resp == nil {
				request.SuggestedCategories = categories
			}
			// Generate page template.
			template = templates.EditSubscription(request)
			pageTitle = templates.GeneratePageTitle("Editing " + request.GetNickname())
		case models.SubscriptionTypeSearch:
			// Editing SearchSubscription.
			request := &models.SearchSubscriptionRequest{
				Customisation: subscription.Customisation,
				Settings:      subscription.Settings,
				Search:        subscription.SearchData.Search,
			}
			request.Search.ID = subscription.GetID()
			// Get any extra subscription info for subscription filters.
			if len(request.Search.Subscriptions) > 0 {
				subscriptions, err := models.GetSubscriptions(ctx,
					models.GetSubscriptionsByIDs(request.Search.Subscriptions...),
				)
				if err != nil {
					return &models.APIError{
						InternalError: fmt.Errorf("get subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to edit subscription",
							"This might be a temporary issue, please try again.",
						),
					}
				}
				ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			}
			// Generate page template.
			template = templates.EditSearchSubscription(request)
			pageTitle = templates.GeneratePageTitle("Editing " + request.Customisation.Nickname)
		case models.SubscriptionTypeGroup:
			// Editing SearchSubscription.
			request := &models.GroupSubscriptionRequest{
				Customisation:  subscription.Customisation,
				Settings:       subscription.Settings,
				Subscriptions:  subscription.GroupData.Subscriptions,
				SubscriptionID: subscription.GetID(),
			}
			subscriptions, err := models.GetSubscriptions(ctx,
				models.GetSubscriptionsByIDs(request.Subscriptions...),
			)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("get subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to edit subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			// Generate page template.
			template = templates.EditGroupSubscription(request)
			pageTitle = templates.GeneratePageTitle("Editing " + request.Customisation.Nickname)
		}
		ctx = templates.PageTitleToCtx(ctx, pageTitle)
		renderPage(wrapContent(req.WithContext(ctx), template)).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SaveSubscription handles saving the edits made by a user to a subscription.
func SaveSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Get the subscription.
		subscription, err := models.GetSubscription(req.Context(), id)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get subscription: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}

		// Generate the appropriate subscription edit request.
		switch models.SubscriptionType(req.FormValue("subscription_type")) {
		case models.SubscriptionTypeFeed:
			request, valid, err := forms.DecodeForm[*models.EditSubscriptionRequest](req)
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			subscription.Customisation.Nickname = request.GetNickname()
			subscription.Customisation.Categories = request.GetCategories()
			subscription.Settings.ShowFullArticleContent = request.ShowFullArticleContent
			subscription.FeedData.ArticleFilters.Text = request.ArticleFilters.Text
			subscription.FeedData.ArticleFilters.Authors = request.ArticleFilters.Authors
			subscription.FeedData.ArticleFilters.Categories = request.ArticleFilters.Categories
		case models.SubscriptionTypeSearch:
			request, valid, err := forms.DecodeForm[*models.SearchSubscriptionRequest](req)
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode search subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			subscription.Customisation = request.Customisation
			subscription.Settings = request.Settings
			subscription.SearchData.Search = request.Search
		case models.SubscriptionTypeGroup:
			request, valid, err := forms.DecodeForm[*models.GroupSubscriptionRequest](req)
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode group subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			subscription.Customisation = request.Customisation
			subscription.Settings = request.Settings
			subscription.GroupData.Subscriptions = request.Subscriptions
		}

		// Process any uploaded thumbnail image.
		thumbnail, err := processThumbnail(req, subscription.GetID())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		if thumbnail != "" {
			subscription.Customisation.ImageURL = thumbnail
		}

		// Update the subscription object.
		_, err = models.UpdateSubscriptions(req.Context(), subscription)
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("update subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save subscription",
					"This might be a temporary issue, please try again.",
				),
			}
		}
		renderPartial(templates.EditSubscriptionSuccessNotification(subscription)).ServeHTTP(res, req)

		return nil
	})).ServeHTTP
}

// AddFeedSubscription handles adding a new subscription to a feed.
func AddFeedSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		ctx := templates.PageTitleToCtx(req.Context(), "Add subscription")
		switch req.Method {
		case http.MethodGet:
			template := templates.AddFeedSubscription(&models.AddFeedSubscriptionRequest{})
			renderPage(wrapContent(req.WithContext(ctx), template)).ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.AddFeedSubscriptionRequest](req.WithContext(ctx))
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode add subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}

			// Process requests.
			resultsCh := make(chan models.AddFeedSubscriptionResult)
			var wg sync.WaitGroup
			wg.Go(func() {
				models.ProcessSubscriptionRequest(ctx, request, resultsCh)
			})
			// Wait for all request processing to complete.
			go func() {
				defer close(resultsCh)
				wg.Wait()
			}()
			result := <-resultsCh
			// Process results
			if result.Error != nil {
				switch result.Message.Status {
				case models.UserMessageStatusError:
					slogctx.FromCtx(ctx).Error("Error occurred during subscription request processing.",
						slog.String("url", result.Request.GetURL()),
						slog.Any("error", result.Error),
					)
				case models.UserMessageStatusWarning:
					fallthrough
				default:
					slogctx.FromCtx(ctx).Warn("Warning occurred during subscription request processing.",
						slog.String("url", result.Request.GetURL()),
						slog.Any("error", result.Error),
					)
				}
			} else {
				err = models.CreateFeedSubscriptions(ctx, &result)
				if err != nil {
					return &models.APIError{
						InternalError: fmt.Errorf("add subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to add subscription",
							"This might be a temporary issue, please try again.",
						),
					}
				}
			}

			renderPartial(templates.Notification(result.Message, 0)).ServeHTTP(res, req.WithContext(ctx))
		}
		return nil
	})).ServeHTTP
}

// AddSearchSubscription handles adding a new search subscription.
func AddSearchSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode add search subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			// If the search request has subscription filters, get subscription details.
			ctx := req.Context()
			if len(request.Subscriptions) > 0 {
				subscriptions, err := models.GetSubscriptions(req.Context(),
					models.GetSubscriptionsByIDs(request.Subscriptions...),
				)
				if err != nil {
					return &models.APIError{
						InternalError: fmt.Errorf("get subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to add subscription",
							"This might be a temporary issue, please try again.",
						),
					}
				}
				ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			}
			template := templates.AddSearchSubscription(&models.SearchSubscriptionRequest{Search: *request})
			ctx = templates.PageTitleToCtx(ctx, "Add search subscription")
			renderPage(
				wrapContent(req.WithContext(ctx), template),
			).ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.SearchSubscriptionRequest](req)
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode search subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			err = models.CreateSearchSubscriptions(req.Context(), request)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("create search subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			renderPartial(
				templates.Notification(models.NewSuccessMessage("Search Subscription Created!", ""), 0),
			).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// AddGroupSubscription handles adding a new group subscription.
func AddGroupSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			template := templates.AddGroupSubscription(&models.GroupSubscriptionRequest{})
			ctx := templates.PageTitleToCtx(req.Context(), "Add group subscription")
			renderPage(
				wrapContent(req.WithContext(ctx), template),
			).ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			// Decode request.
			request, valid, err := forms.DecodeForm[*models.GroupSubscriptionRequest](req)
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode group subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			// Generate subscription metadata from request.
			subscription, err := models.NewGroupSubscription(req.Context(), request)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("new group subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			// Validate subscription.
			if err = subscription.Valid(); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("validate group subscription: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			// Add subscriptions
			if err := models.AddSubscriptions(req.Context(), subscription); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("add subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			// Render notification.
			renderPartial(
				templates.Notification(models.NewSuccessMessage("Search Subscription Created!", ""), 0),
			).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ImportSubscriptions handles assisting the user with importing subscriptions from an external source.
func ImportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			template := templates.ImportSubscriptions()
			ctx := templates.PageTitleToCtx(req.Context(), "Import subscriptions")
			renderPage(
				wrapContent(req.WithContext(ctx), template),
			).ServeHTTP(res, req.WithContext(ctx))
		// POST: process import.
		case http.MethodPost:
			// Extract OPML file.
			opmlData, err := forms.DecodeMultipartFile(req, "source")
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("decode opml: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Failed to read OPML file",
						"The OPML could not be read. Is it a valid OPML file? Please check the contents, correct any issues and try again.",
					),
				}
			}
			opmlFile := &models.OPMLFile{FileUpload: opmlData}
			// Generate subscription requests from OPML file contents.
			requests, err := opmlFile.GenerateRequests()
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("generate subscription requests: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Failed to extract subscriptions from OPML file.",
						"There was a problem reading the individual feed entries in the OPML file. Please check the contents, correct any issues and try again.",
					),
				}
			}

			// Process requests.
			resultsCh := make(chan models.AddFeedSubscriptionResult)
			var wg sync.WaitGroup
			for request := range slices.Values(requests) {
				wg.Go(func() {
					models.ProcessSubscriptionRequest(req.Context(), request, resultsCh)
				})
			}
			// Wait for all request processing to complete.
			go func() {
				defer close(resultsCh)
				wg.Wait()
			}()
			results := make([]*models.AddFeedSubscriptionResult, 0, len(requests))
			// Process results
			for result := range resultsCh {
				results = append(results, &result)
			}
			// Create the subscriptions for any results that don't already indicate an error.
			err = models.CreateFeedSubscriptions(req.Context(), slices.Collect(models.FilterSlice(results,
				func(r *models.AddFeedSubscriptionResult) bool {
					if r.Message != nil && r.Message.Status != models.UserMessageStatusError &&
						r.Message.Status != models.UserMessageStatusWarning {
						return true
					}
					return false
				},
			))...)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("create subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Failed to import.",
						"The backend produced an error. This might be temporary, please try again.",
					),
				}
			}
			// Display all results.
			msg := models.NewSuccessMessage(
				"OPML import complete.",
				"Please consult the results and check for any issues.",
			)
			template := templ.Join(
				templates.ImportResults(results),
				templates.Notification(msg, templates.DefaultNotificationTimeout),
			)
			renderPartial(template).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ExportSubscriptions handles configuring and performing an export of user subscriptions.
func ExportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the user details.
		ctx := templates.PageTitleToCtx(req.Context(), "Export subscriptions")
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return &models.APIError{
				InternalError: fmt.Errorf("get user data: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Failed to export.",
					"The backend produced an error. This might be temporary, please try again.",
				),
			}
		}
		switch {
		// GET: show import modal.
		case chi.RouteContext(ctx).RoutePattern() == "/user/export":
			renderPage(
				wrapContent(req.WithContext(ctx), templates.ExportSubscriptions()),
			).ServeHTTP(res, req.WithContext(ctx))
		case chi.RouteContext(ctx).RoutePattern() == "/user/export/opml":
			// Get all subscriptions.
			request := &models.ListRequest{
				Filters: models.NewListDisplayFilters(),
			}
			subscriptions, _, err := models.FilterSubscriptions(ctx, request)
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("filter subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Failed to export.",
						"The backend produced an error. This might be temporary, please try again.",
					),
				}
			}
			// Create outlines for all subscriptions.
			outlines := make([]opml.Outline, 0, len(subscriptions))
			for subscription := range slices.Values(subscriptions) {
				if subscription.GetSubscriptionType() == models.SubscriptionTypeFeed {
					outlines = append(
						outlines,
						*opml.NewSubscriptionOutline(subscription.Customisation.Nickname, subscription.FeedData.URL,
							opml.WithHTMLURL(subscription.FeedData.URL),
							opml.WithOutlineTitle(subscription.Customisation.Nickname),
						),
					)
				}
			}
			// Generate the opml file from the outlines.
			title := config.AppName + " subscriptions export for " + user.GetNickname()
			opmlExport := opml.NewOPML(
				opml.WithTitle(title),
				opml.WithOutlines(outlines...),
			)
			// Marshal the opml file and convert to a byte reader.
			data, err := xml.Marshal(opmlExport)
			data = []byte(xml.Header + string(data))
			if err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("create opml: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Failed to export.",
						"The backend produced an error. This might be temporary, please try again.",
					),
				}
			}
			// Serve the opml content via http.ServeContent.
			res.Header().Set("Content-Type", "text/x-opml+xml; charset=utf-8")
			filename := config.AppName + "-Export.opml"
			res.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeContent(res, req.WithContext(ctx), filename, time.Now(), bytes.NewReader(data))
		}
		return nil
	})).ServeHTTP
}

// AdjustSubscriptionCategories handles adding and removing categories from a subscription, either when editing or
// adding.
func AdjustSubscriptionCategories() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodPost: // Add category.
			// Parse form values.
			if err := req.ParseForm(); err != nil {
				return &models.APIError{
					InternalError: fmt.Errorf("parse form: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Adjust categories failed.",
						"The backend produced an error. This might be temporary, please try again.",
					),
				}
			}
			currentCategories := req.PostForm["user_categories"]
			inputName := req.FormValue("inputName")
			// Only add a category if it isn't already added.
			if category := req.FormValue("category"); category == "" ||
				(currentCategories != nil && slices.Contains(currentCategories, category)) ||
				inputName == "" {
				res.WriteHeader(http.StatusNoContent)
			} else {
				renderPartial(templates.AddCategory(req.URL.Path, inputName, category)).ServeHTTP(res, req)
			}
		case http.MethodDelete: // Remove a category.
			res.WriteHeader(http.StatusOK)
		default: // Unsupported, do nothing.
			res.WriteHeader(http.StatusNoContent)
		}
		return nil
	})).ServeHTTP
}

func processThumbnail(req *http.Request, objectID string) (string, error) {
	const maxThumbnailSize = 1000000 // Max thumbnail size is 1 MB.

	// Get any uploaded image.
	image, err := forms.DecodeMultipartFile(req, models.ParamThumbnail)
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		return "", fmt.Errorf("parse thumbnail data: %w", err)
	}
	if image.GetSize() > maxThumbnailSize {
		return "", fmt.Errorf("parse thumbnail data: %w", models.ErrFileTooLarge)
	}

	// If the user uploaded a new avatar, process it.
	if image != nil {
		thumbnailCache, err := loadThumbnailCache()
		if err != nil {
			return "", fmt.Errorf("load thumbnail cache: %w", err)
		}
		// Generate a unique ID for the avatar image in the cache using the user ID.
		imageFileID := strconv.FormatUint(xxhash.Sum64String(objectID+"thumbnail"), 10)
		// Read the uploaded data and store in the cache.
		imageData, err := io.ReadAll(image.Data)
		if err != nil {
			return "", fmt.Errorf("read thumbnail data: %w", err)
		}
		thumbnailCache.Set(req.Context(), imageFileID, imageData)
		// Construct a new full URL to the uploaded avatar on the local server.
		baseURL := os.Getenv("FORAGD_BASEURL")
		return baseURL + "/img/subscription/" + imageFileID, nil
	}

	return "", nil
}
