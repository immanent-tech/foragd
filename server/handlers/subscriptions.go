// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListSubscriptions handles fetching subscriptions based on the given page filters and displaying them. When the
// request method is GET (i.e. initial page load), the subscriptions are shown in a grid layout. When the request method
// is POST (i.e. pagination request), the subscriptions are shown as a list.
func ListSubscriptions(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.Append(parseFilters, setCacheControl).
		ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
			filters := models.PageFiltersFromCtx(req.Context(), req.URL.Path)
			pagination := req.FormValue(models.ParamPagination)
			// Redirect to include query parameters in address bar.
			if req.Method == http.MethodGet && len(req.URL.Query()) == 0 {
				if IsHTMX(req) {
					res.Header().Set(htmx.HeaderPushURL, req.URL.Path+"?"+filters.QueryString())
				} else {
					http.Redirect(res, req, req.URL.Path+"?"+filters.QueryString(), http.StatusSeeOther)
				}
			}
			var (
				subscriptions models.Subscriptions
				err           error
				template      templ.Component
				pageTitle     string
			)
			// Remove any subscription filters if this is a history restore request (i.e. back button clicked).
			if htmx.IsHistoryRestoreRequest(req) {
				filters.Subscriptions = nil
			}
			// Get subscriptions matching filters.
			subscriptions, pagination, err = api.FilterSubscriptions(req.Context(), filters, pagination)
			if err != nil && !errors.Is(err, elastic.ErrNotFound) {
				msg := models.NewErrorMessage(
					"Server could not complete request!",
					"This might be temporary, please try again.",
				)
				switch req.Method {
				case http.MethodGet:
					renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
				case http.MethodPost:
					template = templates.ServerErrorNotification(msg)
					renderPartial(template).ServeHTTP(res, req)
				}
				return models.NewAPIError(
					fmt.Errorf("unable to list subscriptions: %w", err),
					http.StatusInternalServerError,
				)
			}
			// Render appropriate content.
			switch req.Method {
			case http.MethodGet:
				template = templates.SubscriptionsGrid(pagination, subscriptions...)
			case http.MethodPost:
				if len(subscriptions) > 0 {
					template = templates.Subscriptions(pagination, subscriptions...)
				} else {
					res.WriteHeader(http.StatusNoContent)
					return nil
				}
			}
			// Choose rendering method based on method (get = page, post = partial).
			switch req.Method {
			case http.MethodGet:
				renderPage(template, templates.GeneratePageTitle(pageTitle)).ServeHTTP(res, req)
			case http.MethodPost:
				renderPartial(template).ServeHTTP(res, req)
			}
			return nil
		})).ServeHTTP
}

// MarkSubscription handles marking a subscription as read/unread and updates the UI accordingly.
func MarkSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request values.
		subscriptionID := chi.URLParam(req, models.ParamSubscriptionID)
		request := &models.MarkSubscriptionsRequest{
			Subscriptions: []models.SubscriptionID{subscriptionID},
			Mark:          models.Mark(chi.URLParam(req, models.ParamMark)),
			View:          models.View(req.FormValue(models.ParamView)),
		}
		err := request.Valid()
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}

		// Mark subscription.
		err = api.MarkSubscriptions(req.Context(), request.Mark, request.Subscriptions...)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to mark subscription",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
		}

		// Determine the URL the request came from.
		currentURL, found := htmx.GetCurrentURL(req)
		if !found {
			err = SetRedirect(res, HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
			})
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage(
							"Unable to mark subscription",
							"This might be a temporary error, please try again.",
						),
					),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
			}
		}
		if strings.Contains(currentURL, "/list/articles") {
			// If the current URL is /list/articles, return to /list/subscriptions.
			err = SetRedirect(res, HXLocationRequest{
				Path:   "/list/subscriptions",
				Target: templates.ContentID.Target(),
				Values: models.PageFiltersFromCtx(req.Context(), "/list/subscriptions").Values(),
			})
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage(
							"Unable to mark subscription",
							"This might be a temporary error, please try again.",
						),
					),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
			}
			res.WriteHeader(http.StatusOK)
		} else {
			// Else swap content apprpriately.
			switch request.View {
			case models.ViewRead, models.ViewUnread:
				res.Header().Set(htmx.HeaderReswap, "delete transition:true")
				res.WriteHeader(http.StatusOK)
			case models.ViewAll:
				subscription, err := api.GetSubscription(req.Context(), request.Subscriptions[0], elastic.GetSubscriptionsDynamicInfo(true))
				if err != nil {
					res.Header().Add(htmx.HeaderReswap, "none")
					renderPartial(
						templates.ServerErrorNotification(
							models.NewErrorMessage("Unable to mark subscription", "This might be a temporary error, please try again.")),
					).ServeHTTP(res, req)
					return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
				}
				res.Header().Set(htmx.HeaderReswap, "outerHTML transition:true")
				renderPartial(templates.SubscriptionCard(subscription)).ServeHTTP(res, req)
			}
		}
		return nil
	})).ServeHTTP
}

// MarkSubscriptions handles marking a list of subscriptions.
func MarkSubscriptions(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Decode request parameters.
		request, valid, err := forms.DecodeForm[*models.MarkSubscriptionsRequest](req)
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to mark subscriptions.",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions: %w", err), http.StatusInternalServerError)
		}
		if !valid {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to mark subscriptions.",
						"This might be a temporary error, please try again.",
					),
				),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions: %w", err), http.StatusUnprocessableEntity)
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
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to mark objects!", "")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions failed: %w", err), http.StatusInternalServerError)
		}

		// Mark subscriptions.
		err = api.MarkSubscriptions(req.Context(), request.Mark, request.Subscriptions...)
		if err != nil {
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to mark objects!", "")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("mark subscriptions failed: %w", err), http.StatusInternalServerError)
		}
		res.WriteHeader(http.StatusOK)
		return nil
	})).ServeHTTP
}

// RemoveSubscription handles removing (unsubscribing) from a subscription.
func RemoveSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		request := &models.RemoveSubscriptionRequest{
			SubscriptionID: chi.URLParam(req, models.ParamSubscriptionID),
			Nickname:       req.FormValue("nickname"),
		}
		err := request.Valid()
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Server could not complete request",
					"This might be temporary, please try again.",
				),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		switch req.FormValue("confirmed") {
		case "false":
			renderPartial(templates.RemoveSubscriptionModal(request)).ServeHTTP(res, req)
		case "true":
			err = api.RemoveSubscriptions(req.Context(), request.SubscriptionID)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(
					templates.ServerErrorNotification(
						models.NewErrorMessage(
							"Unable to unsubscribe to "+request.Nickname,
							"This might be a temporary error, please try again.",
						),
					),
				).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("remove subscription: %w", err), http.StatusInternalServerError)
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
func EditSubscription(api *elastic.API) http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Get the subscription.
		subscription, err := api.GetSubscription(req.Context(), id)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to edit subscription", "Data in invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		var template templ.Component
		var pageTitle string
		ctx := req.Context()
		switch subscription.GetSubscriptionType() {
		case models.SubscriptionTypeFeed:
			// Convert metadata into edit request data.
			request := &models.EditSubscriptionRequest{
				SubscriptionID:         id,
				Nickname:               subscription.Customisation.Nickname,
				Categories:             subscription.Customisation.Categories,
				ShowFullArticleContent: subscription.Settings.ShowFullArticleContent,
				ArticleFilters:         subscription.FeedData.ArticleFilters,
			}
			// Get top categories across items in subscription feed and add as suggested categories for the
			// subscription.
			categories, resp := models.GetArticleTopCategories(ctx, api, subscription.FeedData.GetFeedID())
			if resp == nil {
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
				subscriptions, err := api.GetSubscriptions(ctx,
					elastic.GetSubscriptionsByIDs(request.Search.Subscriptions...),
				)
				if err != nil {
					res.Header().Add(htmx.HeaderReswap, "none")
					renderPartial(templates.ServerErrorNotification(
						models.NewErrorMessage(
							"Unable to add subscription",
							"Data is invalid. Please check your inputs and try again.",
						),
					)).ServeHTTP(res, req.WithContext(ctx))
					return models.NewAPIError(
						fmt.Errorf("add search subscription: %w: %w", ErrInvalidRequestParams, err),
						http.StatusUnprocessableEntity,
					)
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
			subscriptions, err := api.GetSubscriptions(ctx,
				elastic.GetSubscriptionsByIDs(request.Subscriptions...),
			)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to add subscription",
						"Data is invalid. Please check your inputs and try again.",
					),
				)).ServeHTTP(res, req.WithContext(ctx))
				return models.NewAPIError(
					fmt.Errorf("add search subscription: %w: %w", ErrInvalidRequestParams, err),
					http.StatusUnprocessableEntity,
				)
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			// Generate page template.
			template = templates.EditGroupSubscription(request)
			pageTitle = templates.GeneratePageTitle("Editing " + request.Customisation.Nickname)
		}
		renderPage(template, pageTitle).ServeHTTP(res, req.WithContext(ctx))
		return nil
	})).ServeHTTP
}

// SaveSubscription handles saving the edits made by a user to a subscription.
func SaveSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Get the subscription.
		subscription, err := api.GetSubscription(req.Context(), id)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save subscription", "Data in invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				http.StatusUnprocessableEntity,
			)
		}
		switch models.SubscriptionType(req.FormValue("subscription_type")) {
		case models.SubscriptionTypeFeed:
			request, valid, err := forms.DecodeForm[*models.EditSubscriptionRequest](req)
			if err != nil || !valid {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "Data is invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
					http.StatusUnprocessableEntity,
				)
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
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "Data is invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
					http.StatusUnprocessableEntity,
				)
			}
			subscription.Customisation = request.Customisation
			subscription.Settings = request.Settings
			subscription.SearchData.Search = request.Search
		}

		_, err = api.UpdateSubscriptions(req.Context(), subscription)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage(
					"Unable to save subscription",
					"This might be a temporary problem, please try again.",
				),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		renderPartial(templates.EditSubscriptionSuccessNotification(subscription)).ServeHTTP(res, req)

		return nil
	})).ServeHTTP
}

// AddFeedSubscription handles adding a new subscription to a feed.
func AddFeedSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			template := templates.AddFeedSubscription(&models.AddFeedSubscriptionRequest{})
			renderPage(template, templates.GeneratePageTitle("Add Subscription")).ServeHTTP(res, req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.AddFeedSubscriptionRequest](req)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to add subscription", "Data is invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
					http.StatusUnprocessableEntity,
				)
			}

			// Process requests.
			resultsCh := make(chan models.AddFeedSubscriptionResult)
			var wg sync.WaitGroup
			wg.Go(func() {
				api.ProcessSubscriptionRequest(req.Context(), request, resultsCh)
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
					slogctx.FromCtx(req.Context()).Error("Error occurred during subscription request processing.",
						slog.String("url", result.Request.GetURL()),
						slog.Any("error", result.Error),
					)
				case models.UserMessageStatusWarning:
					fallthrough
				default:
					slogctx.FromCtx(req.Context()).Warn("Warning occurred during subscription request processing.",
						slog.String("url", result.Request.GetURL()),
						slog.Any("error", result.Error),
					)
				}
			} else {
				err = api.CreateFeedSubscriptions(req.Context(), &result)
				if err != nil {
					res.Header().Add(htmx.HeaderReswap, "none")
					msg := models.NewErrorMessage("Failed to create subscription.", "The backend produced an error. This might be temporary, please try again.")
					renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
					return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusInternalServerError)
				}
			}

			renderPartial(templates.Notification(&result.Message, 0)).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// AddSearchSubscription handles adding a new search subscription.
func AddSearchSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to add subscription",
						"Data is invalid. Please check your inputs and try again.",
					),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("add search subscription: %w: %w", ErrInvalidRequestParams, err),
					http.StatusUnprocessableEntity,
				)
			}
			// If the search request has subscription filters, get subscription details.
			ctx := req.Context()
			if len(request.Subscriptions) > 0 {
				subscriptions, err := api.GetSubscriptions(req.Context(),
					elastic.GetSubscriptionsByIDs(request.Subscriptions...),
				)
				if err != nil {
					res.Header().Add(htmx.HeaderReswap, "none")
					renderPartial(templates.ServerErrorNotification(
						models.NewErrorMessage(
							"Unable to add subscription",
							"Data is invalid. Please check your inputs and try again.",
						),
					)).ServeHTTP(res, req)
					return models.NewAPIError(
						fmt.Errorf("add search subscription: %w: %w", ErrInvalidRequestParams, err),
						http.StatusUnprocessableEntity,
					)
				}
				ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			}
			template := templates.AddSearchSubscription(&models.SearchSubscriptionRequest{Search: *request})
			renderPage(
				template,
				templates.GeneratePageTitle("Add A Search Subscription"),
			).ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.SearchSubscriptionRequest](req)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to add subscription",
						"Data is invalid. Please check your inputs and try again.",
					),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("add search subscription: %w: %w", ErrInvalidRequestParams, err),
					http.StatusUnprocessableEntity,
				)
			}
			err = api.CreateSearchSubscriptions(req.Context(), request)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to create subscription.",
					"The backend produced an error. This might be temporary, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("add search subscription: %w", err),
					http.StatusInternalServerError,
				)
			}
			renderPartial(
				templates.Notification(models.NewSuccessMessage("Search Subscription Created!", ""), 0),
			).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// AddGroupSubscription handles adding a new group subscription.
func AddGroupSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			template := templates.AddGroupSubscription(&models.GroupSubscriptionRequest{})
			renderPage(template, templates.GeneratePageTitle("Add A Group Subscription")).ServeHTTP(res, req)
		case http.MethodPost:
			// Decode request.
			request, valid, err := forms.DecodeForm[*models.GroupSubscriptionRequest](req)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage(
						"Unable to add subscription",
						"Data is invalid. Please check your inputs and try again.",
					),
				)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("add group subscription: %w: %w", ErrInvalidRequestParams, err),
					http.StatusUnprocessableEntity,
				)
			}
			// Generate subscription metadata from request.
			subscription, err := models.NewGroupSubscription(req.Context(), request)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to create subscription.",
					"The backend produced an error. This might be temporary, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("add group subscription: %w", err), http.StatusInternalServerError)
			}
			// Validate subscription.
			err = subscription.Valid()
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to create subscription.",
					"The backend produced an error. This might be temporary, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("add group subscription: %w", err), http.StatusInternalServerError)
			}
			// Add subscriptions
			err = api.AddSubscriptions(req.Context(), subscription)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to create subscription.",
					"The backend produced an error. This might be temporary, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("add group subscription: %w", err), http.StatusInternalServerError)
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
func (a *API) ImportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			template := templates.ImportSubscriptions()
			renderPage(template, templates.GeneratePageTitle("Import Subscriptions")).ServeHTTP(res, req)
		// POST: process import.
		case http.MethodPost:
			// Extract OPML file.
			opmlFileUpload, err := forms.DecodeMultipartFile(req, "source")
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to read OPML file",
					"The OPML could not be read. Is it a valid OPML file? Please check the contents, correct any issues and try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable process import request: %w", err),
					http.StatusUnprocessableEntity,
				)
			}
			opmlFile := &models.OPMLFile{
				FileUpload: opmlFileUpload,
			}
			// Generate subscription requests from OPML file contents.
			requests, err := opmlFile.GenerateRequests()
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewWarningMessage(
					"Failed to extract subscriptions from OPML file.",
					"There was a problem reading the individual feed entries in the OPML file. Please check the contents, correct any issues and try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable process import request: %w", err),
					http.StatusInternalServerError,
				)
			}

			// Process requests.
			resultsCh := make(chan models.AddFeedSubscriptionResult)
			var wg sync.WaitGroup
			for request := range slices.Values(requests) {
				wg.Go(func() {
					a.Elastic.ProcessSubscriptionRequest(req.Context(), request, resultsCh)
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
			err = a.Elastic.CreateFeedSubscriptions(req.Context(), slices.Collect(models.FilterSlice(results,
				func(r *models.AddFeedSubscriptionResult) bool {
					if r.Message.Status != models.UserMessageStatusError &&
						r.Message.Status != models.UserMessageStatusWarning {
						return true
					}
					return false
				},
			))...)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to import.",
					"The backend produced an error. This might be temporary, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable process import request: %w", err),
					http.StatusInternalServerError,
				)
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
func (a *API) ExportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the user details.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			msg := models.NewErrorMessage(
				"Unable to load export form",
				"This might be a temporary problem, please try again.",
			)
			template := templ.Join(templates.ExportSubscriptions(), templates.ServerErrorNotification(msg))
			renderPage(template, templates.GeneratePageTitle("Export Subscriptions")).ServeHTTP(res, req)
			return models.NewAPIError(
				fmt.Errorf("unable to retrieve user data: %w", models.ErrNoUserCtx),
				http.StatusInternalServerError,
			)
		}
		switch {
		// GET: show import modal.
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export":
			renderPage(
				templates.ExportSubscriptions(),
				templates.GeneratePageTitle("Export Subscriptions"),
			).ServeHTTP(res, req)
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export/opml":
			// Get all subscriptions.
			filters := models.NewListDisplayFilters()
			subscriptions, _, err := a.Elastic.FilterSubscriptions(req.Context(), &filters, "")
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Error exporting OPML file.",
					"The backend had issues generating the OPML file, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable process import request: %w", err),
					http.StatusInternalServerError,
				)
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
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Error exporting OPML file.",
					"The backend had issues generating the OPML file, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(
					fmt.Errorf("unable process import request: %w", err),
					http.StatusInternalServerError,
				)
			}
			// Serve the opml content via http.ServeContent.
			res.Header().Set("Content-Type", "text/x-opml+xml; charset=utf-8")
			filename := config.AppName + "-Export.opml"
			res.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeContent(res, req, filename, time.Now(), bytes.NewReader(data))
		}
		return nil
	})).ServeHTTP
}

// AdjustSubscriptionCategories handles adding and removing categories from a subscription, either when editing or
// adding.
func AdjustSubscriptionCategories() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodPost: // Add category.
			// Parse form values.
			err := req.ParseForm()
			if err != nil {
				return fmt.Errorf("unable to parse category changes: %w", err)
			}
			currentCategories := req.PostForm["user_categories"]
			category := req.FormValue("category")
			inputName := req.FormValue("inputName")
			// Only add a category if it isn't already added.
			if category == "" || (currentCategories != nil && slices.Contains(currentCategories, category)) ||
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
