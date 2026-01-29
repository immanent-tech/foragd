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
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/server/forms"
	htmxext "github.com/immanent-tech/foragd/web/htmx"
	"github.com/immanent-tech/foragd/web/templates"
)

// ListSubscriptions holds data for generating the subscriptions list page.
type ListSubscriptions struct {
	title    string
	template templ.Component
}

// FullResponse renders a full page (headers, footers and list of subscriptions).
func (p *ListSubscriptions) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

// PartialResponse will either render the list of subscriptions, the controls and update the title/dock/sidebar or, when
// paginating, just the list of subscriptions.
func (p *ListSubscriptions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/list/subscriptions":
		templ.Handler(p.template, templ.WithFragments(templates.ListSubscriptionsFragment)).ServeHTTP(res, req)
		templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
		templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
		templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	case "/list/subscriptions/paginate":
		templ.Handler(p.template, templ.WithFragments(templates.PaginateSubscriptionsFragment)).ServeHTTP(res, req)
	}
}

// HandleListSubscriptions handles displaying a list of subscriptions.
func HandleListSubscriptions() http.HandlerFunc {
	return listHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
			}).ServeHTTP(res, req)
			return
		}

		// Generate request object.
		request := &models.ListRequest{
			Filters:    *models.PageFiltersFromCtx(req.Context(), req.URL.Path),
			Pagination: req.FormValue(models.ParamPagination),
		}
		if err := request.Valid(); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
			}).ServeHTTP(res, req)
			return
		}

		// Redirect to include query parameters in address bar.
		if req.URL.Path != "/list/subscriptions/paginate" {
			switch {
			case htmx.IsHTMX(req):
				res.Header().Set(htmx.HeaderReplaceUrl, req.URL.Path+"?"+request.Filters.QueryString())
			case len(req.URL.Query()) == 0:
				http.Redirect(res, req, req.URL.Path+"?"+request.Filters.QueryString(), http.StatusSeeOther)
			}
		}

		var (
			subscriptions models.Subscriptions
		)

		// Remove any subscription filters if this is a history restore request (i.e. back button clicked).
		if htmx.IsHistoryRestoreRequest(req) {
			request.Filters.Subscriptions = nil
		}

		// Get subscriptions matching filters.
		subscriptions, request.Pagination, err = models.FilterSubscriptions(req.Context(), request)
		if err != nil && !errors.Is(err, elastic.ErrNotFound) {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}

		// Build response object.
		response := &models.ListSubscriptionsResponse{
			User:          *user,
			Filters:       request.Filters,
			Pagination:    request.Pagination,
			Subscriptions: subscriptions,
		}

		// Choose rendering method based on method (get = page, post = partial).
		switch req.Method {
		case http.MethodGet:
			RenderInternalPage(&ListSubscriptions{
				title:    "Subscriptions",
				template: templates.ListSubscriptions(response),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			RenderPartial(&ListSubscriptions{
				title:    "Subscriptions",
				template: templates.ListSubscriptions(response),
			}).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

// HandleMarkSubscription handles marking a subscription as read/unread and updates the UI accordingly.
func HandleMarkSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode request parameters.
		request, _, err := forms.DecodeForm[*models.MarkSubscriptionRequest](req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode mark subscriptions request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark articles.",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Mark subscription.
		if err := models.MarkSubscriptions(req.Context(), request.Mark, request.SubscriptionID); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("mark subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Do extra processing based on the current url.
		if currentURL, found := htmx.GetCurrentURL(req); found {
			if strings.Contains(currentURL, "/list/articles") {
				// On /list/articles, redirect back to subscriptions after marking.
				if err := setRedirect(res, htmxext.HXLocationRequest{
					Path:   "/list/subscriptions",
					Target: templates.ContentID.Target(),
					Values: models.PageFiltersFromCtx(req.Context(), "/list/subscriptions").Values(),
				}); err != nil {
					slogctx.FromCtx(req.Context()).Warn("Unable to set redirect", slog.Any("error", err))
				}
			}
		}

		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// HandleMarkSubscriptions handles marking a list of subscriptions.
func HandleMarkSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Decode request parameters.
		request, valid, err := forms.DecodeForm[*models.MarkSubscriptionsRequest](req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode mark subscription request: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		if !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("validate mark subscription request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Determine what mark to apply from view and where to redirect.
		switch request.View {
		case models.ViewUnread:
			err = setRedirect(res, htmxext.HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
			})
		case models.ViewRead:
			err = setRedirect(res, htmxext.HXLocationRequest{
				Path:   "/home",
				Target: templates.ContentID.Target(),
			})
		default:
			err = setRedirect(res, htmxext.HXLocationRequest{
				Path:   "/list/subscriptions",
				Target: templates.ContentID.Target(),
				Values: models.PageFiltersFromCtx(req.Context(), req.URL.Path).Values(),
			})
		}
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("set redirect: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Mark subscriptions.
		err = models.MarkSubscriptions(req.Context(), request.Mark, request.Subscriptions...)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("mark subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to mark subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// HandleFavoriteSubscription handles managing a favorite subscription for a user.
func HandleFavoriteSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.FavoriteSubscriptionRequest](req)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		if !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("%w: %w", ErrInvalidRequestParams, err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		subscription, err := models.GetSubscription(req.Context(), request.SubscriptionID)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to favorite subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		var favorite bool
		if subscription.IsFavorite() {
			favorite = false
		} else {
			favorite = true
		}

		// Get the subscription state.
		if err := models.UpdateFavoriteSubscription(req.Context(), request.SubscriptionID, favorite); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update favorite subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to add favorite subscription",
					"This might be a temporary error, please try again.",
				),
			}).ServeHTTP(res, req)
		}

		res.WriteHeader(http.StatusOK)
	}).ServeHTTP
}

// HandleRemoveSubscription handles removing (unsubscribing) from a subscription.
func HandleRemoveSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request := &models.RemoveSubscriptionRequest{
			SubscriptionID: chi.URLParam(req, models.ParamSubscriptionID),
			Nickname:       req.FormValue("nickname"),
		}
		if err := request.Valid(); err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("validate remove subscription request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to remove subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		switch req.FormValue("confirmed") {
		case "false":
			renderPartial(templates.NewPartial(templates.RemoveSubscriptionModal(request))).ServeHTTP(res, req)
		case "true":
			if err := models.RemoveSubscriptions(req.Context(), request.SubscriptionID); err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("remove subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to remove subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Show success notification.
			RenderPartial(
				&Notification{
					msg: models.NewSuccessMessage("Unsubscribed from "+request.Nickname, ""),
				},
			).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

// EditSubscription contains the data for rendering a page for editing a subscription.
type EditSubscription struct {
	title    string
	template templ.Component
}

func (p *EditSubscription) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(p.template,
			templates.WithPageTitle(p.title),
		)).ServeHTTP(res, req)
}

func (p *EditSubscription) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(p.template, templ.WithFragments(templates.EditSubscriptionFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
}

// HandleEditSubscription handles presenting the user with a form for editing a subscription.
func HandleEditSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
			}).ServeHTTP(res, req)
			return
		}

		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Get the subscription.
		subscription, err := models.GetSubscription(req.Context(), id)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to edit subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		var template templ.Component
		var pageTitle string
		ctx := req.Context()
		switch subscription.GetSubscriptionType() {
		case models.SubscriptionTypeFeed:
			// Convert metadata into edit request data.
			request := &models.EditSubscriptionRequest{
				User:                   *user,
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
			pageTitle = "Editing " + request.GetNickname()
		case models.SubscriptionTypeSearch:
			// Editing SearchSubscription.
			request := &models.SearchSubscriptionRequest{
				User:          *user,
				Customisation: subscription.Customisation,
				Settings:      subscription.Settings,
				Search:        subscription.SearchData.Search,
			}
			request.Search.SubscriptionID = subscription.GetID()
			// Get any extra subscription info for subscription filters.
			if len(request.Search.Subscriptions) > 0 {
				subscriptions, err := models.GetSubscriptions(ctx,
					models.GetSubscriptionsByIDs(request.Search.Subscriptions...),
				)
				if err != nil {
					HandleInternalError(&models.APIError{
						InternalError: fmt.Errorf("get subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to edit subscription",
							"This might be a temporary issue, please try again.",
						),
					}).ServeHTTP(res, req)
					return
				}
				ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			}
			// Generate page template.
			template = templates.EditSearchSubscription(request)
			pageTitle = "Editing " + request.Customisation.Nickname
		case models.SubscriptionTypeGroup:
			// Editing SearchSubscription.
			request := &models.GroupSubscriptionRequest{
				User:           *user,
				Customisation:  subscription.Customisation,
				Settings:       subscription.Settings,
				Subscriptions:  subscription.GroupData.Subscriptions,
				SubscriptionID: subscription.GetID(),
			}
			subscriptions, err := models.GetSubscriptions(ctx,
				models.GetSubscriptionsByIDs(request.Subscriptions...),
			)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("get subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to edit subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			// Generate page template.
			template = templates.EditGroupSubscription(request)
			pageTitle = "Editing " + request.Customisation.Nickname
		case models.SubscriptionTypeEmail:
			// Editing SearchSubscription.
			request := &models.EditEmailSubscriptionRequest{
				User:           *user,
				Customisation:  subscription.Customisation,
				Settings:       subscription.Settings,
				SubscriptionID: subscription.GetID(),
			}
			template = templates.EditEmailSubscription(request)
			pageTitle = "Editing " + request.Customisation.Nickname
		}
		// Render the page.
		RenderInternalPage(
			&EditSubscription{
				title:    pageTitle,
				template: template,
			},
		).ServeHTTP(res, req.WithContext(ctx))
	}).ServeHTTP
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
		case models.SubscriptionTypeEmail:
			request, valid, err := forms.DecodeForm[*models.EditEmailSubscriptionRequest](req)
			if err != nil || !valid {
				return &models.APIError{
					InternalError: fmt.Errorf("decode email subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}
			}
			subscription.Customisation = request.Customisation
			subscription.Settings = request.Settings
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
		renderPartial(
			templates.NewPartial(
				templates.Notification(
					models.NewSuccessMessage(
						subscription.GetTitle()+" saved", "",
					),
					templates.DefaultNotificationTimeout,
				)),
		).ServeHTTP(res, req)

		return nil
	})).ServeHTTP
}

// AddFeedSubscription handles adding a new subscription to a feed.
func AddFeedSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			template := templates.AddFeedSubscription(&models.AddFeedSubscriptionRequest{})
			renderPage(
				templates.NewPage(
					wrapContent(req, template),
					templates.WithPageTitle("Add Feed Subscription"),
				),
			).ServeHTTP(res, req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.AddFeedSubscriptionRequest](req)
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
				models.ProcessSubscriptionRequest(req.Context(), request, resultsCh)
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
				err = models.CreateFeedSubscriptions(req.Context(), &result)
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

			renderPartial(templates.NewPartial(templates.Notification(result.Message, 0))).ServeHTTP(res, req)
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
			renderPage(
				templates.NewPage(
					wrapContent(req.WithContext(ctx), template),
					templates.WithPageTitle("Add Search Subscription"),
				),
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
				templates.NewPartial(
					templates.Notification(models.NewSuccessMessage("Search Subscription Created!", ""), 0),
				),
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
			renderPage(
				templates.NewPage(
					wrapContent(req, template),
					templates.WithPageTitle("Add Group Subscription"),
				),
			).ServeHTTP(res, req)
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
				templates.NewPartial(
					templates.Notification(models.NewSuccessMessage("Search Subscription Created!", ""), 0),
				),
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
			renderPage(
				templates.NewPage(
					wrapContent(req, template),
					templates.WithPageTitle("Import Subscriptions"),
				),
			).ServeHTTP(res, req)
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
					if r.Message == nil {
						return true
					}
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
			renderPartial(templates.NewPartial(template)).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ExportSubscriptions handles configuring and performing an export of user subscriptions.
func ExportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(notifyOnError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the user details.
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
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			renderPage(
				templates.NewPage(
					wrapContent(req, templates.ExportSubscriptions()),
					templates.WithPageTitle("Export Subscriptions"),
				),
			).ServeHTTP(res, req)
		case http.MethodPost:
			// Get all subscriptions.
			subscriptions, err := models.GetSubscriptions(req.Context())
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
			http.ServeContent(res, req, filename, time.Now(), bytes.NewReader(data))
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
				renderPartial(
					templates.NewPartial(
						templates.AddCategory(req.URL.Path, inputName, category),
					),
				).ServeHTTP(res, req)
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
