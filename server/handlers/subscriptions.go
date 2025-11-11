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

// EditSubscription handles presenting the user with a form for editing a subscription.
func (a *API) EditSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamObjectID)
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			msg := models.NewErrorMessage("Unable to edit subscription", "This might be a temporary problem, please try again.")
			renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle("Error")).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		subscription := user.GetSubscriptionByID(id)
		var template templ.Component
		var pageTitle string
		switch subscription.GetType() {
		case models.SubscriptionTypeFeed:
			// Editing FeedSubscription.
			feedSubscription, err := subscription.Data.AsFeedSubscription()
			if err != nil {
				msg := models.NewErrorMessage("Unable to edit subscription", "This might be a temporary problem, please try again.")
				renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle("Error")).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("edit subscription failed: as feed subscription: %w", err), http.StatusInternalServerError)
			}
			// Convert metadata into edit request data.
			request := &models.EditSubscriptionRequest{
				SubscriptionID:         id,
				Nickname:               subscription.Metadata.Customisation.Nickname,
				Categories:             subscription.Metadata.Customisation.Categories,
				ShowFullArticleContent: subscription.Metadata.Settings.ShowFullArticleContent,
				ArticleFilters:         feedSubscription.ArticleFilters,
			}
			// Get top categories across items in subscription feed and add as suggested categories for the
			// subscription.
			categories, resp := models.GetArticleTopCategories(req.Context(), a.Elastic, feedSubscription.FeedID)
			if resp == nil {
				request.SuggestedCategories = categories
			}
			// Generate page template.
			template = templates.EditSubscription(request)
			pageTitle = templates.GeneratePageTitle("Editing " + request.GetNickname())
		case models.SubscriptionTypeSearch:
			// Editing SearchSubscription.
			searchSubscription, err := subscription.Data.AsSearchSubscription()
			if err != nil {
				msg := models.NewErrorMessage("Unable to edit subscription", "This might be a temporary problem, please try again.")
				renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle("Error")).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("edit subscription failed: as search subscription: %w", err), http.StatusInternalServerError)
			}
			request := &models.SearchSubscriptionRequest{
				Customisation: subscription.Metadata.Customisation,
				Settings:      subscription.Metadata.Settings,
				Search:        searchSubscription.Search,
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
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			msg := models.NewErrorMessage("Unable to save subscription", "This might be a temporary problem, please try again.")
			renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
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
			// Update the subscription.
			feedSubscription := user.GetFeedSubscriptions().GetByID(request.SubscriptionID)
			if feedSubscription == nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "Data in invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}
			feedSubscription.Metadata.Customisation.Nickname = request.GetNickname()
			feedSubscription.Metadata.Customisation.Categories = request.GetCategories()
			feedSubscription.Metadata.Settings.ShowFullArticleContent = request.ShowFullArticleContent
			feedSubscription.ArticleFilters.Text = request.ArticleFilters.Text
			feedSubscription.ArticleFilters.Authors = request.ArticleFilters.Authors
			feedSubscription.ArticleFilters.Categories = request.ArticleFilters.Categories
			err = models.UpdateSubscription(req.Context(), api, feedSubscription)
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "This might be a temporary problem, please try again."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
			}
			renderPartial(templates.EditSubscriptionSuccessNotification(feedSubscription)).ServeHTTP(res, req)
		case models.SubscriptionTypeSearch:
			request, valid, err := forms.DecodeForm[*models.SearchSubscriptionRequest](req)
			if err != nil || !valid {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "Data is invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}
			searchSubscription := user.GetSearchSubscriptions().GetByID(chi.URLParam(req, models.ParamSubscriptionID))
			if searchSubscription == nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "Data in invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}
			searchSubscription.Metadata.Customisation = request.Customisation
			searchSubscription.Metadata.Settings = request.Settings
			searchSubscription.Search = request.Search

			err = models.UpdateSubscription(req.Context(), api, searchSubscription)
			if err != nil {
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to save subscription", "This might be a temporary problem, please try again."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
			}
			renderPartial(templates.EditSubscriptionSuccessNotification(searchSubscription)).ServeHTTP(res, req)
		}
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
			subscriptions, err := models.GetSubscriptions(req.Context(), a.Elastic)
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
				feedSubscription, ok := subscription.(*models.FeedSubscription)
				if ok {
					outlines = append(outlines, *opml.NewSubscriptionOutline(feedSubscription.GetTitle(), feedSubscription.Feed.GetSourceURLs()[0],
						opml.WithHTMLURL(feedSubscription.GetLink()),
						opml.WithOutlineTitle(feedSubscription.GetTitle()),
						opml.WithDescription(feedSubscription.GetDescription()),
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
		valid, err := validation.ValidateStruct(newFeed)
		if !valid || err != nil {
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
	if user.IsSubscribedToFeed(feed.GetID()) {
		subscription := user.GetFeedSubscriptions().GetByFeedID(feed.GetID())
		result.Error = fmt.Errorf("already subscribed")
		result.Message = *models.NewWarningMessage("Already subscribed to feed", fmt.Sprintf("%s %q", subscription.Metadata.Customisation.Nickname, request.GetURL()))
		resultsCh <- result
		return
	}
	// Add the feed details to the result.
	result.Feed = *feed
	// Send the result back through the channel.
	resultsCh <- result
}
