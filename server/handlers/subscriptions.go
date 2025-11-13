// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"context"
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
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
)

// MarkSubscription handles marking a subscription as read/unread and updates the UI accordingly.
func MarkSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Extract request values.
		request := &models.MarkSubscriptionRequest{
			SubscriptionID: chi.URLParam(req, models.ParamSubscriptionID),
			Mark:           models.Mark(chi.URLParam(req, models.ParamMark)),
			View:           models.View(req.FormValue(models.ParamView)),
		}
		err := request.Valid()
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to mark subscription", "This might be a temporary issue, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Mark subscription.
		err = models.MarkSubscriptions(req.Context(), api, request.Mark, request.SubscriptionID)
		if err != nil {
			res.Header().Add(htmx.HeaderReswap, "none")
			renderPartial(
				templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to mark subscription", "This might be a temporary error, please try again.")),
			).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user: %w", err), http.StatusInternalServerError)
		}

		switch request.View {
		case models.ViewRead, models.ViewUnread:
			res.Header().Set(htmx.HeaderReswap, "delete transition:true")
			res.WriteHeader(http.StatusOK)
		case models.ViewAll:
			subscription, err := models.GetSubscription(req.Context(), api, request.SubscriptionID)
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

		return nil
	})).ServeHTTP
}

// EditSubscription handles presenting the user with a form for editing a subscription.
func (a *API) EditSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamObjectID)
		// Get the subscription.
		subscription, err := models.GetSubscription(req.Context(), a.DataAPI(), id)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save subscription", "Data in invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		var template templ.Component
		var pageTitle string
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
			categories, resp := models.GetArticleTopCategories(req.Context(), a.Elastic, subscription.FeedData.GetFeedID())
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
			// Generate page template.
			template = templates.EditSearchSubscription(request)
			pageTitle = templates.GeneratePageTitle("Editing " + request.Customisation.Nickname)
		}
		renderPage(template, pageTitle).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveSubscription handles saving the edits made by a user to a subscription.
func SaveSubscription(api *elastic.API) http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the subscription.
		subscription, err := models.GetSubscription(req.Context(), api, req.FormValue(models.ParamSubscriptionID))
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save subscription", "Data in invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		switch models.SubscriptionType(req.FormValue("subscription_type")) {
		case models.SubscriptionTypeFeed:
			request, valid, err := forms.DecodeForm[*models.EditSubscriptionRequest](req)
			if err != nil || !valid {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "Data is invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
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
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}
			subscription.Customisation = request.Customisation
			subscription.Settings = request.Settings
			subscription.SearchData.Search = request.Search
		}

		_, err = api.UpdateSubscriptions(req.Context(), subscription)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save subscription", "This might be a temporary problem, please try again."),
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
			// Retrieve user object.
			user, err := models.UserFromCtx(req.Context())
			if err != nil {
				msg := models.NewErrorMessage("Unable to edit subscription", "This might be a temporary problem, please try again.")
				renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle("Error")).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
			}

			request, valid, err := forms.DecodeForm[*models.AddFeedSubscriptionRequest](req)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to add subscription", "Data is invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}

			// Process requests.
			resultsCh := make(chan models.AddFeedSubscriptionResult)
			var wg sync.WaitGroup
			wg.Go(func() {
				processSubscriptionRequest(req.Context(), api, user, request, resultsCh)
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
				err = models.CreateFeedSubscriptions(req.Context(), api, &result)
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
					models.NewErrorMessage("Unable to add subscription", "Data is invalid. Please check your inputs and try again."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}
			template := templates.AddSearchSubscription(&models.SearchSubscriptionRequest{Search: *request})
			renderPage(template, templates.GeneratePageTitle("Add A Search Subscription")).ServeHTTP(res, req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.SearchSubscriptionRequest](req)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to add subscription", "Data is invalid. Please check your inputs and try again."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}
			err = models.CreateSearchSubscriptions(req.Context(), api, request)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage("Failed to create subscription.", "The backend produced an error. This might be temporary, please try again.")
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusInternalServerError)
			}
			renderPartial(templates.Notification(models.NewSuccessMessage("Search Subscription Created!", ""), 0)).ServeHTTP(res, req)
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
			// Retrieve user object.
			user, err := models.UserFromCtx(req.Context())
			if err != nil {
				msg := models.NewErrorMessage("Unable to edit subscription", "This might be a temporary problem, please try again.")
				renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle("Error")).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
			}
			// Extract OPML file.
			opmlFileUpload, err := forms.DecodeMultipartFile(req, "source")
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to read OPML file",
					"The OPML could not be read. Is it a valid OPML file? Please check the contents, correct any issues and try again.")
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusUnprocessableEntity)
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
				return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusInternalServerError)
			}

			// Process requests.
			resultsCh := make(chan models.AddFeedSubscriptionResult)
			var wg sync.WaitGroup
			for request := range slices.Values(requests) {
				wg.Go(func() {
					processSubscriptionRequest(req.Context(), a.Elastic, user, request, resultsCh)
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
				if result.Error != nil {
					switch result.Message.Status {
					case models.UserMessageStatusError:
						slogctx.FromCtx(req.Context()).Error("Error occurred during subscription request processing.",
							slog.String("url", result.Request.GetURL()),
							slog.Any("error", result.Error),
						)
						continue
					case models.UserMessageStatusWarning:
						fallthrough
					default:
						slogctx.FromCtx(req.Context()).Warn("Warning occurred during subscription request processing.",
							slog.String("url", result.Request.GetURL()),
							slog.Any("error", result.Error),
						)
						continue
					}
				}
				results = append(results, &result)
			}
			// Create the subscriptions.
			err = models.CreateFeedSubscriptions(req.Context(), a.Elastic, results...)
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage("Failed to import.", "The backend produced an error. This might be temporary, please try again.")
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusInternalServerError)
			}
			// Display results.
			msg := models.NewSuccessMessage("OPML import complete.", "Please consult the results and check for any issues.")
			template := templ.Join(templates.ImportResults(results), templates.Notification(msg, 10*time.Second))
			renderPartial(template).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ExportSubscriptions handles configuring and performing an export of user subscriptions.
func (a *API) ExportSubscriptions() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the user details.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			msg := models.NewErrorMessage("Unable to load export form", "This might be a temporary problem, please try again.")
			template := templ.Join(templates.ExportSubscriptions(), templates.ServerErrorNotification(msg))
			renderPage(template, templates.GeneratePageTitle("Export Subscriptions")).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		switch {
		// GET: show import modal.
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export":
			renderPage(templates.ExportSubscriptions(), templates.GeneratePageTitle("Export Subscriptions")).ServeHTTP(res, req)
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export/opml":
			// Get all subscriptions.
			filters := models.NewListDisplayFilters()
			subscriptions, _, err := models.FilterSubscriptions(req.Context(), a.Elastic, &filters, "")
			if err != nil {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Error exporting OPML file.",
					"The backend had issues generating the OPML file, please try again.",
				)
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusInternalServerError)
			}
			// Create outlines for all subscriptions.
			outlines := make([]opml.Outline, 0, len(subscriptions))
			for subscription := range slices.Values(subscriptions) {
				if subscription.GetSubscriptionType() == models.SubscriptionTypeFeed {
					outlines = append(outlines, *opml.NewSubscriptionOutline(subscription.Customisation.Nickname, subscription.FeedData.URL,
						opml.WithHTMLURL(subscription.FeedData.URL),
						opml.WithOutlineTitle(subscription.Customisation.Nickname),
					))
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
				return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusInternalServerError)
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
			if category == "" || (currentCategories != nil && slices.Contains(currentCategories, category)) || inputName == "" {
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

func processSubscriptionRequest(ctx context.Context, api *elastic.API, user *models.User, request *models.AddFeedSubscriptionRequest, resultsCh chan models.AddFeedSubscriptionResult) {
	slogctx.FromCtx(ctx).Debug("Processing subscription request.",
		slog.String("url", request.GetURL()))
	result := models.AddFeedSubscriptionResult{
		Request: *request,
	}
	// Try to match request URL to an existing feed
	feed, err := models.MatchRequestToFeed(ctx, api, request)
	if err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			result.Error = err
			result.Message = *models.NewErrorMessage("Unable to determine existing subscription status", "The backend produced an error. This might be temporary, please try again.")
			resultsCh <- result
			return
		}
	}
	// If no existing feed, create a new one.
	if feed == nil {
		slogctx.FromCtx(ctx).Debug("No existing feed, creating new one.",
			slog.String("url", request.GetURL()),
		)
		newFeed, err := models.NewFeedFromURL(ctx, request.GetURL())
		if err != nil {
			result.Error = err
			result.Message = *models.NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = validation.Validate.Struct(newFeed)
		if err != nil {
			result.Error = err
			result.Message = *models.NewErrorMessage("Unable to create subscription", fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL()))
			resultsCh <- result
			return
		}
		err = models.CreateFeed(ctx, api, newFeed)
		if err != nil {
			result.Error = err
			result.Message = *models.NewErrorMessage("Unable to create new feed for subscription", "The backend produced an error. This might be temporary, please try again.")
			resultsCh <- result
			return
		}
		slogctx.FromCtx(ctx).Debug("Created new feed for request.",
			slog.String("name", newFeed.GetTitle()),
			slog.String("urls", strings.Join(newFeed.GetSourceURLs(), ",")),
		)
		feed = newFeed
	}
	// Check if user already subscribed.

	subscribed, err := models.IsUserSubscribedToFeed(ctx, api, feed.GetID())
	if err != nil {
		result.Error = err
		result.Message = *models.NewErrorMessage("Unable to check for existing subscription", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	if subscribed {
		result.Error = fmt.Errorf("already subscribed")
		result.Message = *models.NewWarningMessage("Already subscribed to feed", "")
		resultsCh <- result
		return
	}
	// Add the feed details to the result.
	result.Feed = *feed
	// Send the result back through the channel.
	resultsCh <- result
}
