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
		metadata := user.GetSubscriptionMetadata().GetByID(id)
		// Convert metadata into edit request data.
		request := &models.EditSubscriptionRequest{
			SubscriptionID:         id,
			Nickname:               metadata.Customisation.Nickname,
			Categories:             metadata.Customisation.Categories,
			ShowFullArticleContent: metadata.Settings.ShowFullArticleContent,
			ArticleFilters:         metadata.Customisation.ArticleFilters,
		}
		// Get top categories across items in subscription feed and add as suggested categories for the
		// subscription.
		categories, resp := models.GetArticleTopCategories(req.Context(), a.Elastic, metadata.GetFeedID())
		if resp == nil {
			request.SuggestedCategories = categories
		}
		// Generate page template.
		template := templates.EditSubscription(request)
		renderPage(template, templates.GeneratePageTitle("Editing "+request.GetNickname())).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveSubscription handles saving the edits made by a user to a subscription.
func (a *API) SaveSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			msg := models.NewErrorMessage("Unable to save subscription", "This might be a temporary problem, please try again.")
			renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
		}
		request, valid, err := forms.DecodeForm[*models.EditSubscriptionRequest](req)
		if err != nil || !valid {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save subscription", "Data is invalid."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
		}
		// Update the subscription metadata.
		metadata := user.GetSubscriptionMetadata().GetByID(request.SubscriptionID)
		metadata.Customisation.Nickname = request.GetNickname()
		metadata.Customisation.Categories = request.GetCategories()
		metadata.Settings.ShowFullArticleContent = request.ShowFullArticleContent
		metadata.Customisation.ArticleFilters.Text = request.ArticleFilters.Text
		metadata.Customisation.ArticleFilters.Authors = request.ArticleFilters.Authors
		metadata.Customisation.ArticleFilters.Categories = request.ArticleFilters.Categories
		err = user.UpdateSubscription(metadata)
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save subscription", "This might be a temporary problem, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update subscription object: %w", err), http.StatusInternalServerError)
		}
		// Update the user.
		err = a.DataAPI().UpdateUser(req.Context(), user.GetID(), map[string]any{
			"subscriptions": user.GetSubscriptionMetadata(),
		})
		if err != nil {
			renderPartial(templates.ServerErrorNotification(
				models.NewErrorMessage("Unable to save subscription", "This might be a temporary problem, please try again."),
			)).ServeHTTP(res, req)
			return models.NewAPIError(fmt.Errorf("unable to update user data: %w", err), http.StatusInternalServerError)
		}
		renderPartial(templates.EditSubscriptionSuccessNotification(metadata)).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddSubscription handles adding a new subscription requested by the user.
func (a *API) AddSubscription() http.HandlerFunc {
	return defaultHandlerChain.ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			template := templates.AddSubscription(&models.SubscriptionRequest{})
			renderPage(template, templates.GeneratePageTitle("Add Subscription")).ServeHTTP(res, req)
		case http.MethodPost:
			// Retrieve user object.
			user, err := models.UserFromCtx(req.Context())
			if err != nil {
				msg := models.NewErrorMessage("Unable to edit subscription", "This might be a temporary problem, please try again.")
				renderPage(templates.ErrorPage(msg), templates.GeneratePageTitle("Error")).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable to retrieve user data: %w", err), http.StatusInternalServerError)
			}

			request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				renderPartial(templates.ServerErrorNotification(
					models.NewErrorMessage("Unable to add subscription", "Data is invalid."),
				)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("%w: %w", ErrInvalidRequestParams, err), http.StatusUnprocessableEntity)
			}

			// Process requests.
			resultsCh := make(chan models.SubscriptionResult)
			var wg sync.WaitGroup
			wg.Go(func() {
				processSubscriptionRequest(req.Context(), a.Elastic, user, request, resultsCh)
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
			}

			renderPartial(templates.Notification(&result.Message, 0)).ServeHTTP(res, req)
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

			opmlFile := &models.OPMLFile{}
			opmlFile, valid, err := forms.DecodeMultipartFile(req, "source", opmlFile)
			if err != nil || !valid {
				res.Header().Add(htmx.HeaderReswap, "none")
				msg := models.NewErrorMessage(
					"Failed to read OPML file",
					"The OPML could not be read. Is it a valid OPML file? Please check the contents, correct any issues and try again.")
				renderPartial(templates.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(fmt.Errorf("unable process import request: %w", err), http.StatusUnprocessableEntity)
			}
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
			resultsCh := make(chan models.SubscriptionResult)
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
			results := make([]*models.SubscriptionResult, 0, len(requests))
			// Process results
			for result := range resultsCh {
				results = append(results, &result)
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
				}
			}
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
				outlines = append(outlines, *opml.NewSubscriptionOutline(subscription.GetTitle(), subscription.Feed.GetSourceURLs()[0],
					opml.WithHTMLURL(subscription.GetLink()),
					opml.WithOutlineTitle(subscription.GetTitle()),
					opml.WithDescription(subscription.GetDescription()),
				))
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
			// Only add a category if it isn't already added.
			if category == "" || (currentCategories != nil && slices.Contains(currentCategories, category)) {
				res.WriteHeader(http.StatusNoContent)
			} else {
				renderPartial(templates.AddCategory(req.URL.Path, category)).ServeHTTP(res, req)
			}
		case http.MethodDelete: // Remove a category.
			res.WriteHeader(http.StatusOK)
		default: // Unsupported, do nothing.
			res.WriteHeader(http.StatusNoContent)
		}
		return nil
	})).ServeHTTP
}

func processSubscriptionRequest(ctx context.Context, api *elastic.API, user *models.User, request *models.SubscriptionRequest, resultsCh chan models.SubscriptionResult) {
	slogctx.FromCtx(ctx).Debug("Processing subscription request.",
		slog.String("url", request.GetURL()))
	result := models.SubscriptionResult{
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
		subscription := user.GetSubscriptionMetadata().GetByFeedID(feed.GetID())
		result.Error = fmt.Errorf("already subscribed")
		result.Message = *models.NewWarningMessage("Already subscribed to feed", fmt.Sprintf("%s %q", subscription.Customisation.Nickname, request.GetURL()))
		resultsCh <- result
		return
	}
	// Add the feed details to the result.
	result.Feed = *feed
	// Create the subscription.
	subscription, err := models.CreateSubscription(ctx, api, request, feed)
	if err != nil {
		result.Error = err
		result.Message = *models.NewErrorMessage("Unable to create user subscription", "The backend produced an error. This might be temporary, please try again.")
		resultsCh <- result
		return
	}
	// Add subscription details to the result.
	result.Subscription = *subscription
	result.Message = *models.NewSuccessMessage("Subscription Created: "+feed.GetTitle(), "Articles will be fetched shortly...")
}
