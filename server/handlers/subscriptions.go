// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/server/session"
	htmxext "github.com/immanent-tech/foragd/web/htmx"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/partials"
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
		res.Header().Set(htmx.HeaderPushURL, req.URL.String())
		templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
		templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
		templ.Handler(templates.SideBar(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
		templ.Handler(templates.Dock(templ.Attributes{"hx-swap-oob": "true"})).ServeHTTP(res, req)
	case "/list/subscriptions/paginate":
		templ.Handler(p.template, templ.WithFragments(templates.PaginateFragment)).ServeHTTP(res, req)
	}
}

// HandleListSubscriptions handles displaying a list of subscriptions.
func HandleListSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		filters := getListSubscriptionsFilters(req)

		// Generate request object.
		request := &models.ListRequest{
			Filters:    *filters,
			Pagination: new(req.FormValue(models.ParamPagination)),
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
			err           error
		)

		// Remove any subscription filters if this is a history restore request (i.e. back button clicked).
		if htmx.IsHistoryRestoreRequest(req) {
			request.Filters.Subscriptions = nil
		}

		// Get subscriptions matching filters.
		var next models.Pagination
		subscriptions, next, err = models.FilterSubscriptions(req.Context(), request)
		if err != nil && !errors.Is(err, models.ErrNotFound) {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("unable to list subscriptions: %w", err),
				StatusCode:    http.StatusInternalServerError,
			}).ServeHTTP(res, req)
			return
		}
		request.Pagination = &next

		// Choose rendering method based on method (get = page, post = partial).
		switch req.Method {
		case http.MethodGet:
			RenderInternalPage(&ListSubscriptions{
				title: "Subscriptions",
				template: templates.ListSubscriptions(&models.ListSubscriptionsResponse{
					Filters:       request.Filters,
					Pagination:    *request.Pagination,
					Subscriptions: subscriptions,
				}),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			RenderPartial(&ListSubscriptions{
				title: "Subscriptions",
				template: templates.ListSubscriptions(&models.ListSubscriptionsResponse{
					Filters:       request.Filters,
					Pagination:    *request.Pagination,
					Subscriptions: subscriptions,
				}),
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
					Values: getListSubscriptionsFilters(req).Values(),
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
				Values: getListSubscriptionsFilters(req).Values(),
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
			RenderPartial(
				&Modal{
					template: templates.RemoveSubscriptionModal(request),
				},
			).ServeHTTP(res, req)
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
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(p.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(p.title)).ServeHTTP(res, req)
}

// HandleEditSubscription handles presenting the user with a form for editing a subscription.
func HandleEditSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Get the existingSubscription.
		existingSubscription, err := models.GetSubscription(req.Context(), id)
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
		switch existingSubscription.GetSubscriptionType() {
		case models.SubscriptionTypeFeed:
			// Convert metadata into edit request data.
			request := &models.FeedSubscriptionRequest{
				SubscriptionID: id,
				Customisation:  existingSubscription.Customisation,
				Settings:       &existingSubscription.Settings,
				ArticleFilters: existingSubscription.FeedData.ArticleFilters,
			}
			// Get top suggestedCategories across items in subscription feed and add as suggested suggestedCategories for the
			// subscription.
			request.SuggestedCategories = getSubscriptionCategorySuggestions(
				req.Context(),
				[]models.FeedID{existingSubscription.FeedData.GetFeedID()},
				existingSubscription.Customisation.Categories,
			)
			// Generate page template.
			template = templates.EditFeedSubscription(request)
			pageTitle = "Editing " + existingSubscription.GetTitle()
		case models.SubscriptionTypeSearch:
			// Editing SearchSubscription.
			request := &models.SearchSubscriptionRequest{
				Customisation: existingSubscription.Customisation,
				Settings:      &existingSubscription.Settings,
				Search:        existingSubscription.SearchData.Search,
			}
			// Get suggested categories from existing subscriptions.
			categoryCounts, err := models.GetCategoriesForSubscriptions(req.Context())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to get category suggestions from existing subscriptions.",
					slog.Any("error", err),
				)
			}
			request.SuggestedCategories = categoryCounts.Limit(10).GetCategories()

			request.Search.SubscriptionID = new(existingSubscription.GetID())
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
			pageTitle = "Editing " + request.Customisation.GetNickname()
		case models.SubscriptionTypeGroup:
			subscriptions, err := models.GetSubscriptions(
				req.Context(),
				models.GetSubscriptionsByIDs(existingSubscription.GroupData.Subscriptions...),
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

			// Create the request with details from the group subscription.
			request := &models.GroupSubscriptionRequest{
				Customisation:  existingSubscription.Customisation,
				Settings:       new(existingSubscription.Settings),
				Subscriptions:  make(map[models.SubscriptionID]string),
				SubscriptionID: new(existingSubscription.GetID()),
				ArticleFilters: existingSubscription.GroupData.ArticleFilters,
			}
			// Populate the subscriptions data in the request.
			for subscription := range slices.Values(subscriptions) {
				request.Subscriptions[subscription.GetID()] = subscription.GetTitle()
			}
			ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			// Get top suggestedCategories across items in subscription feed and add as suggested suggestedCategories
			// for the subscription.
			request.SuggestedCategories = getSubscriptionCategorySuggestions(
				req.Context(),
				subscriptions.GetFeedIDs(),
				subscriptions.GetCategories(),
			)
			// Get suggested suggested subscriptions.
			suggestedSubscriptions, err := models.GetSubscriptions(req.Context())
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("get subscriptions: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to edit group subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			request.SuggestedSubscriptions = suggestedSubscriptions.FilterByType(models.SubscriptionTypeFeed)

			// Generate page template.
			template = templates.EditGroupSubscription(request)
			pageTitle = "Editing " + request.Customisation.GetNickname()
		case models.SubscriptionTypeEmail:
			// Editing SearchSubscription.
			request := &models.EditEmailSubscriptionRequest{
				Customisation:  existingSubscription.Customisation,
				Settings:       new(existingSubscription.Settings),
				SubscriptionID: existingSubscription.GetID(),
			}
			// Get suggested categories from existing subscriptions.
			categoryCounts, err := models.GetCategoriesForSubscriptions(req.Context())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to get category suggestions from existing subscriptions.",
					slog.Any("error", err),
				)
			}
			request.SuggestedCategories = categoryCounts.Limit(10).GetCategories()

			template = templates.EditEmailSubscription(request)
			pageTitle = "Editing " + request.Customisation.GetNickname()
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

// HandleSaveSubscription handles saving the edits made by a user to a subscription.
func HandleSaveSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, models.ParamSubscriptionID)
		// Get the subscription.
		subscription, err := models.GetSubscription(req.Context(), id)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get subscription: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to save subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Generate the appropriate subscription edit request.
		switch models.SubscriptionType(req.FormValue("subscription_type")) {
		case models.SubscriptionTypeFeed:
			request, valid, err := forms.DecodeMultiPartForm[*models.FeedSubscriptionRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			subscription.Customisation = request.Customisation
			if request.Settings != nil {
				subscription.Settings = *request.Settings
			}
			if request.ArticleFilters != nil {
				if subscription.FeedData.ArticleFilters == nil {
					subscription.FeedData.ArticleFilters = &models.SubscriptionArticleFilters{}
				}
				if request.ArticleFilters.Text != nil {
					subscription.FeedData.ArticleFilters.Text = request.ArticleFilters.Text
				}
				if request.ArticleFilters.Authors != nil {
					subscription.FeedData.ArticleFilters.Authors = request.ArticleFilters.Authors
				}
				if request.ArticleFilters.Categories != nil {
					subscription.FeedData.ArticleFilters.Categories = request.ArticleFilters.Categories
				}
			}
		case models.SubscriptionTypeSearch:
			request, valid, err := forms.DecodeMultiPartForm[*models.SearchSubscriptionRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode search subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			subscription.Customisation = request.Customisation
			if request.Settings != nil {
				subscription.Settings = *request.Settings
			}
			subscription.SearchData.Search = request.Search
		case models.SubscriptionTypeGroup:
			request, valid, err := forms.DecodeMultiPartForm[*models.GroupSubscriptionRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode group subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}

			subscription.Customisation = request.Customisation
			if request.Settings != nil {
				subscription.Settings = *request.Settings
			}
			subscription.GroupData.Subscriptions = slices.Collect(maps.Keys(request.Subscriptions))
			if request.ArticleFilters != nil {
				if subscription.GroupData.ArticleFilters == nil {
					subscription.GroupData.ArticleFilters = &models.SubscriptionArticleFilters{}
				}
				if request.ArticleFilters.Text != nil {
					subscription.GroupData.ArticleFilters.Text = request.ArticleFilters.Text
				}
				if request.ArticleFilters.Authors != nil {
					subscription.GroupData.ArticleFilters.Authors = request.ArticleFilters.Authors
				}
				if request.ArticleFilters.Categories != nil {
					subscription.GroupData.ArticleFilters.Categories = request.ArticleFilters.Categories
				}
			}
		case models.SubscriptionTypeEmail:
			request, valid, err := forms.DecodeMultiPartForm[*models.EditEmailSubscriptionRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode email subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to save subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			subscription.Customisation = request.Customisation
			if request.Settings != nil {
				subscription.Settings = *request.Settings
			}
		}

		// Process any uploaded thumbnail image.
		thumbnail, err := processThumbnail(req, subscription.GetID())
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		if thumbnail != "" {
			subscription.Customisation.ImageURL = new(thumbnail)
		}

		// Update the subscription object.
		subscription.UpdatedAt = new(time.Now().UTC())
		_, err = models.UpdateSubscriptions(req.Context(), subscription)
		if err != nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("update subscription: %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to save subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		RenderPartial(
			&Notification{
				msg: models.NewSuccessMessage(
					subscription.GetTitle()+" saved", "",
				),
			},
		).ServeHTTP(res, req)
	}).ServeHTTP
}

// AddSubscription contains the data for rendering a page for editing a subscription.
type AddSubscription struct {
	title    string
	template templ.Component
}

func (h *AddSubscription) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle(h.title),
		)).ServeHTTP(res, req)
}

func (h *AddSubscription) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle(h.title)).ServeHTTP(res, req)
}

// HandleAddFeedSubscription handles adding a new subscription to a feed.
func HandleAddFeedSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			// Get suggested categories from existing subscriptions.
			categoryCounts, err := models.GetCategoriesForSubscriptions(req.Context())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to get category suggestions from existing subscriptions.",
					slog.Any("error", err),
				)
			}
			suggestedCategories := categoryCounts.Limit(10).GetCategories()
			res.Header().Set(htmx.HeaderPushURL, req.URL.String())
			RenderInternalPage(
				&AddSubscription{
					title: "Add Feed Subscription",
					template: templates.AddFeedSubscription(
						&models.FeedSubscriptionRequest{SuggestedCategories: suggestedCategories},
					),
				},
			).ServeHTTP(res, req)
		case http.MethodPost:
			request, valid, err := forms.DecodeMultiPartForm[*models.FeedSubscriptionRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode add feed subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add feed subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}

			// Process the request.
			results := models.BulkImportFeeds(req.Context(), *request)
			if results[0].Error != nil {
				HandleInternalError(results[0].Error).ServeHTTP(res, req)
				return
			}

			RenderPartial(&Notification{
				msg: models.NewSuccessMessage(
					"Subscription Created!",
					results[0].Subscription.GetTitle(),
				),
			}).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

// HandleAddSearchSubscription handles adding a new search subscription.
func HandleAddSearchSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			request, valid, err := forms.DecodeForm[*models.SearchRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode add search subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// If the search request has subscription filters, get subscription details.
			ctx := req.Context()
			if len(request.Subscriptions) > 0 {
				subscriptions, err := models.GetSubscriptions(req.Context(),
					models.GetSubscriptionsByIDs(request.Subscriptions...),
				)
				if err != nil {
					HandleInternalError(&models.APIError{
						InternalError: fmt.Errorf("get subscription: %w", err),
						StatusCode:    http.StatusInternalServerError,
						UserMessage: models.NewErrorMessage(
							"Unable to add subscription",
							"This might be a temporary issue, please try again.",
						),
					}).ServeHTTP(res, req)
					return
				}
				ctx = models.SubscriptionsToCtx(ctx, subscriptions)
			}
			// Get suggested categories from existing subscriptions.
			categoryCounts, err := models.GetCategoriesForSubscriptions(req.Context())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to get category suggestions from existing subscriptions.",
					slog.Any("error", err),
				)
			}
			suggestedCategories := categoryCounts.Limit(10).GetCategories()
			// Render form.
			RenderInternalPage(
				&AddSubscription{
					title: "Add Search Subscription",
					template: templates.AddSearchSubscription(
						models.NewSearchSubscriptionRequest(*request, suggestedCategories),
					),
				},
			).ServeHTTP(res, req.WithContext(ctx))
		case http.MethodPost:
			request, valid, err := forms.DecodeMultiPartForm[*models.SearchSubscriptionRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode search subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			err = models.CreateSearchSubscriptions(req.Context(), request)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("create search subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
			}
			RenderPartial(
				&Notification{
					msg: models.NewSuccessMessage("Search Subscription Created!", ""),
				},
			).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

func HandleSuggestSubscriptionForSearch() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.GetSubscriptionsSuggestionRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Error("Could not suggest subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		subscriptions, err := models.GetSubscriptionSuggestions(
			req.Context(),
			request.Text,
			10,
			models.IgnoreSubscriptions(request.IgnoredSubscriptions...),
			// 	slices.Collect(maps.Keys(request.IgnoredSubscriptions))...,
			// ))
		)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Could not suggest subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		RenderPartial(&PartialTemplate{
			template: partials.SubscriptionSuggestions(subscriptions),
		}).ServeHTTP(res, req)
	}).ServeHTTP
}

func HandleAddSubscriptionToSearch() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.AddSubscriptionToSearchRequest](req)
		if err != nil || !valid {
			slogctx.FromCtx(req.Context()).Error("Could not suggest subscriptions.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusNoContent)
			return
		}
		for key, value := range request.SuggestedSubscriptions {
			if value == request.SelectedSubscription {
				RenderPartial(&PartialTemplate{
					template: templates.AddSearchSubscriptionFilter(&models.AddSubscriptionSearchFilterRequest{
						InputName:        "search.subscriptions",
						SubscriptionID:   key,
						SubscriptionName: value,
					}),
				}).ServeHTTP(res, req)
				return
			}
		}
		res.WriteHeader(http.StatusNoContent)
	}).ServeHTTP
}

// HandleAddGroupSubscription handles adding a new group subscription.
func HandleAddGroupSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case http.MethodGet:
			// Get suggested categories from existing subscriptions.
			categoryCounts, err := models.GetCategoriesForSubscriptions(req.Context())
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Unable to get category suggestions from existing subscriptions.",
					slog.Any("error", err),
				)
			}
			suggestedCategories := categoryCounts.Limit(10).GetCategories()

			// Get suggested suggested subscriptions.
			suggestedSubscriptions, err := models.GetSubscriptions(req.Context())
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("get subscriptions: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add group subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			suggestedSubscriptions = suggestedSubscriptions.FilterByType(models.SubscriptionTypeFeed)

			RenderInternalPage(
				&AddSubscription{
					title: "Add Group Subscription",
					template: templates.AddGroupSubscription(
						models.NewGroupSubscriptionRequest(suggestedSubscriptions, suggestedCategories),
					),
				},
			).ServeHTTP(res, req)
		case http.MethodPost:
			// Decode request.
			request, valid, err := forms.DecodeMultiPartForm[*models.GroupSubscriptionRequest](req)
			if err != nil || !valid {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode group subscription request: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Generate subscription metadata from request.
			subscription, err := models.NewGroupSubscription(req.Context(), request)
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("new group subscription: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Validate subscription.
			if err = subscription.Valid(); err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("validate group subscription: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Add subscriptions
			if err := models.AddSubscriptions(req.Context(), subscription); err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("add subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Unable to add subscription",
						"This might be a temporary issue, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Render notification.
			RenderPartial(
				&Notification{
					msg: models.NewSuccessMessage("Group Subscription Created!", ""),
				},
			).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

func HandleAddSubscriptionToGroup() http.HandlerFunc {
	return alice.New().ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Parse add subscription to group request.
		request, valid, err := forms.DecodeForm[*models.AddSubscriptionToGroupRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("add subscription to group request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to add subscription to group",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// Ignore request to add subscription that is already in the group.
		if slices.Contains(slices.Collect(maps.Values(request.ExistingSubscriptions)), request.SuggestionText) {
			HandleInternalError(&models.APIError{
				InternalError: errors.New("subscription already in group"),
				StatusCode:    http.StatusConflict,
				UserMessage: models.NewWarningMessage(
					"Not adding subscription",
					"Already in group.",
				),
			}).ServeHTTP(res, req)
			return
		}
		for subscriptionID, subscriptionName := range request.Suggestions {
			if subscriptionName == request.SuggestionText {
				RenderPartial(&PartialTemplate{
					template: templates.AddSubscriptionToGroup(
						subscriptionID,
						subscriptionName,
					),
				}).ServeHTTP(res, req)
				return
			}
		}
		res.WriteHeader(http.StatusNoContent)
	}).ServeHTTP
}

// ImportSubscriptions contains the data for rendering a page for importing subscriptions.
type ImportSubscriptions struct {
	template templ.Component
}

func (h *ImportSubscriptions) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle("Import Subscriptions"),
		)).ServeHTTP(res, req)
}

func (h *ImportSubscriptions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Import Subscriptions")).ServeHTTP(res, req)
}

type ImportSubscriptionsResults struct {
	template templ.Component
}

func (h *ImportSubscriptionsResults) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(h.template).ServeHTTP(res, req)
}

// HandleImportSubscriptions handles assisting the user with importing subscriptions from an external source.
func HandleImportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			RenderInternalPage(&ImportSubscriptions{
				template: templates.ImportSubscriptions(),
			}).ServeHTTP(res, req)
		// POST: process import.
		case http.MethodPost:
			// Extract OPML file.
			opmlData, err := forms.DecodeMultipartFile(req, "source")
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("decode opml: %w", err),
					StatusCode:    http.StatusUnprocessableEntity,
					UserMessage: models.NewErrorMessage(
						"Failed to read OPML file",
						"The OPML could not be read. Is it a valid OPML file? Please check the contents, correct any issues and try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			opmlFile := &models.OPMLFile{FileUpload: opmlData}
			// Generate subscription requests from OPML file contents.
			requests, err := opmlFile.GenerateRequests()
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("generate subscription requests: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Failed to extract subscriptions from OPML file.",
						"There was a problem reading the individual feed entries in the OPML file. Please check the contents, correct any issues and try again.",
					),
				}).ServeHTTP(res, req)
				return
			}

			// Perform bulk import.
			results := models.BulkImportFeeds(req.Context(), requests...)

			// Display all results.
			RenderPartial(&ImportSubscriptionsResults{
				template: templates.ImportSubscriptionsResults(results),
			}).ServeHTTP(res, req)
			// Display notification.
			RenderPartial(&Notification{
				msg: models.NewSuccessMessage(
					"OPML import complete.",
					"Please consult the results and check for any issues.",
				),
			}).ServeHTTP(res, req)
		}
	}).ServeHTTP
}

// ExportSubscriptions contains the data for rendering a page for exporting subscriptions.
type ExportSubscriptions struct {
	template templ.Component
}

func (h *ExportSubscriptions) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(h.template,
			templates.WithPageTitle("Import Subscriptions"),
		)).ServeHTTP(res, req)
}

func (h *ExportSubscriptions) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(h.template, templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Import Subscriptions")).ServeHTTP(res, req)
}

// HandleExportSubscriptions handles configuring and performing an export of user subscriptions.
func HandleExportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		// Get the user details.
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("get user data: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Failed to export.",
					"The backend produced an error. This might be temporary, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			RenderInternalPage(
				&ExportSubscriptions{
					template: templates.ExportSubscriptions(),
				},
			).ServeHTTP(res, req)
		case http.MethodPost:
			// Get all subscriptions.
			subscriptions, err := models.GetSubscriptions(req.Context())
			if err != nil {
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("filter subscriptions: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Failed to export.",
						"The backend produced an error. This might be temporary, please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
			// Create outlines for all subscriptions.
			outlines := make([]opml.Outline, 0, len(subscriptions))
			for subscription := range slices.Values(subscriptions) {
				if subscription.GetSubscriptionType() == models.SubscriptionTypeFeed {
					outlines = append(
						outlines,
						*opml.NewSubscriptionOutline(subscription.Customisation.GetNickname(), subscription.GetLink(),
							opml.WithHTMLURL(subscription.GetLink()),
							opml.WithOutlineTitle(subscription.Customisation.GetNickname()),
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
				HandleInternalError(&models.APIError{
					InternalError: fmt.Errorf("create opml: %w", err),
					StatusCode:    http.StatusInternalServerError,
					UserMessage: models.NewErrorMessage(
						"Failed to export.",
						"The backend produced an error. This might be temporary, please try again.",
					),
				}).ServeHTTP(res, req)
			}
			// Serve the opml content via http.ServeContent.
			res.Header().Set("Content-Type", "text/x-opml+xml; charset=utf-8")
			filename := config.AppName + "-Export.opml"
			res.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeContent(res, req, filename, time.Now(), bytes.NewReader(data))
		}
	}).ServeHTTP
}

// HandleSubscriptionCategories handles adding and removing categories from a subscription, either when editing or
// adding.
func HandleSubscriptionCategories() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(func(res http.ResponseWriter, req *http.Request) {
		request, valid, err := forms.DecodeForm[*models.AddCategoryToSubscriptionRequest](req)
		if err != nil || !valid {
			HandleInternalError(&models.APIError{
				InternalError: fmt.Errorf("decode add subscription category request: %w", err),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to category to subscription",
					"This might be a temporary issue, please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Ignore existing categories.
		if slices.Contains(request.ExistingCategories, request.Category) {
			res.WriteHeader(http.StatusNoContent)
			return
		}
		RenderPartial(&PartialTemplate{template: templates.AddCategory(request.Category)}).ServeHTTP(res, req)
	}).ServeHTTP
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
		imageFileID := strconv.FormatUint(xxh3.Hash([]byte(objectID+"thumbnail")), 10)
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

func getSubscriptionCategorySuggestions(
	ctx context.Context,
	feedIDs []models.FeedID,
	excludedCategories []models.Category,
) []models.Category {
	var suggestions []models.Category
	// Get top suggestedCategories across items in subscription feed and add as suggested suggestedCategories for the
	// subscription.
	topCategoriesQuery := query.Bool(
		query.Filter(
			// Must match any of the given feed IDs.
			query.Terms("feed_id", feedIDs...),
		),
		query.MustNot(
			query.Terms(
				"categories.raw",
				slices.Concat(models.CommonCategoryFilters, excludedCategories)...,
			),
		),
	)
	if suggestedCategories, resp := models.GetArticleTopCategories(ctx, topCategoriesQuery); resp == nil {
		suggestedCategories = slices.Collect(
			models.FilterSlice(suggestedCategories, func(category models.Category) bool {
				return !slices.Contains(models.CommonCategoryFilters, category)
			}),
		)
		suggestions = suggestedCategories
	}

	return suggestions
}

func getListSubscriptionsFilters(req *http.Request) *models.ListFilters {
	// Parse and process filters.
	filters, valid, err := forms.DecodeForm[*models.ListFilters](req)
	switch {
	case err != nil:
		slogctx.FromCtx(req.Context()).Warn("Unable to subscription filters. Using filters from session.",
			slog.Any("error", err),
			slog.Any("filters", filters),
		)
		// Try to restore filters from session.
		filters = session.GetListSubscriptionFiltersFromSession(req.Context())
	case !valid:
		slogctx.FromCtx(req.Context()).Warn("Invalid subscription filters. Creating new filters.")
		session.StoreListSubscriptionFiltersInSession(req.Context(), models.NewListDisplayFilters())
	default:
		session.StoreListSubscriptionFiltersInSession(req.Context(), *filters)
	}

	return filters
}
