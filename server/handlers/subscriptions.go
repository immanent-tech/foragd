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
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/immanent-tech/go-syndication/opml"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/bulk"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/layouts"
	"github.com/immanent-tech/foragd/web/templates/partials"
)

// EditSubscription handles presenting the user with a form for editing a subscription.
func (a *API) EditSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve the subscription ID from the URL parameter.
		id := chi.URLParam(req, models.ParamObjectID)
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to edit subscription: %w", err)
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
		template := layouts.EditSubscription(request)
		renderPage(template, templates.GeneratePageTitle("Editing "+request.GetNickname())).ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// SaveSubscription handles saving the edits made by a user to a subscription.
func (a *API) SaveSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Retrieve user object.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to save subscription: %w", err)
		}
		request, valid, err := forms.DecodeForm[*models.EditSubscriptionRequest](req)
		if err != nil || !valid {
			renderPage(layouts.EditSubscription(request), "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Update the subscription metadata.
		metadata := user.GetSubscriptionMetadata().GetByID(request.SubscriptionID)
		metadata.Customisation.Nickname = request.GetNickname()
		metadata.Customisation.Categories = request.GetCategories()
		metadata.Settings.ShowFullArticleContent = request.ShowFullArticleContent
		metadata.Customisation.ArticleFilters.Authors = request.ArticleFilters.Authors
		metadata.Customisation.ArticleFilters.Categories = request.ArticleFilters.Categories
		err = user.UpdateSubscription(metadata)
		if err != nil {
			msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
			template := templ.Join(layouts.EditSubscription(request), partials.Notification(msg, 0))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		// Update the user.
		err = a.DataAPI().UpdateUser(req.Context(), map[string]any{
			"subscriptions": user.GetSubscriptionMetadata(),
		})
		if err != nil {
			msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
			template := templ.Join(layouts.EditSubscription(request), partials.Notification(msg, 0))
			renderPage(template, "").ServeHTTP(res, req)
			return models.NewAPIError(err, http.StatusUnprocessableEntity)
		}
		template := templ.Join(layouts.EditSubscription(request), layouts.EditSubscriptionSuccessNotification(metadata))
		renderPage(template, "").ServeHTTP(res, req)
		return nil
	})).ServeHTTP
}

// AddSubscription handles adding a new subscription requested by the user.
func (a *API) AddSubscription() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		case http.MethodGet:
			template := layouts.AddSubscription(&models.SubscriptionRequest{})
			renderPage(template, templates.GeneratePageTitle("Add Subscription")).ServeHTTP(res, req)
		case http.MethodPost:
			request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
			if err != nil || !valid {
				renderPage(layouts.AddSubscription(request), "").ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			requests := addSubscriptionRequests{
				request: &models.Feed{},
			}
			// Match the request to either and existing or new feed.
			result, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
				template := templ.Join(layouts.AddSubscription(request), partials.Notification(msg, 0))
				renderPage(template, "").ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			// If results returned from matching is non-nil, something went wrong.
			if result[request] != nil {
				request.URLErr = errors.New("unable to find a feed at given URL")
				// msg := models.NewErrorMessage("Unable to find a feed!", "Please try again with a different URL.")
				// template := templ.Join(layouts.AddSubscription(request), partials.Notification(msg, 0))
				template := layouts.AddSubscription(request)
				renderPage(template, "").ServeHTTP(res, req)
				res.WriteHeader(http.StatusUnprocessableEntity)
				return nil
			}
			// Create the new subscription.
			createResult, err := requests.createNewSubscriptions(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
				template := templ.Join(layouts.AddSubscription(request), partials.Notification(msg, 0))
				renderPage(template, "").ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			} else {
				result = createResult
			}
			if result[request].Message.Status != models.UserMessageStatusSuccess {
				msg := models.NewErrorMessage("An error occurred trying to save the subscription", "Please try again.")
				template := templ.Join(layouts.AddSubscription(request), partials.Notification(msg, 0))
				renderPage(template, "").ServeHTTP(res, req)
				return models.NewAPIError(errors.New(result[request].Message.String()), http.StatusUnprocessableEntity)
			}
			template := layouts.AddSubscriptionSuccess(result[request])

			renderPage(template, "").ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ImportSubscriptions handles assisting the user with importing subscriptions from an external source.
func (a *API) ImportSubscriptions() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		switch req.Method {
		// GET: show import modal.
		case http.MethodGet:
			template := layouts.ImportSubscriptions()
			renderPage(template, templates.GeneratePageTitle("Import Subscriptions")).ServeHTTP(res, req)
		// POST: process import.
		case http.MethodPost:
			requests := make(addSubscriptionRequests)
			opmlFile := &models.OPMLFile{}
			opmlFile, valid, err := forms.DecodeMultipartFile(req, "source", opmlFile)
			if err != nil || !valid {
				msg := models.NewErrorMessage(
					"Failed to read OPML file",
					"The OPML could not be read. Is it a valid OPML file? Please check the contents, correct any issues and try again.")
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			r, err := opmlFile.GenerateRequests()
			if err != nil {
				msg := models.NewWarningMessage(
					"Failed to extract subscriptions from OPML file.",
					"There was a problem reading the individual feed entries in the OPML file. Please check the contents, correct any issues and try again.",
				)
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			for newRequest := range slices.Values(r) {
				requests[newRequest] = &models.Feed{}
			}
			matchResults, err := requests.matchFeedsToSubscriptionRequests(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage(
					"Error processing OPML file.",
					"The backend had issues processing the OPML file and adding subscriptions, please try again.",
				)
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			createResults, err := requests.createNewSubscriptions(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage(
					"Error processing OPML file.",
					"The backend had issues processing the OPML file and adding subscriptions, please try again.",
				)
				renderPartial(partials.Notification(msg, 0)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
			}
			maps.Copy(createResults, matchResults)
			msg := models.NewInfoMessage(
				"OPML import complete.", "Please consult the results and check for any issues.",
			)
			template := templ.Join(layouts.ImportResults(createResults), partials.Notification(msg, 10*time.Second))
			renderPartial(template).ServeHTTP(res, req)
		}
		return nil
	})).ServeHTTP
}

// ExportSubscriptions handles configuring and performing an export of user subscriptions.
func (a *API) ExportSubscriptions() http.HandlerFunc {
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
		// Get the user details.
		user, err := models.UserFromCtx(req.Context())
		if err != nil {
			return fmt.Errorf("unable to export subscription: %w", err)
		}
		switch {
		// GET: show import modal.
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export":
			renderPage(layouts.ExportSubscriptions(), templates.GeneratePageTitle("Export Subscriptions")).ServeHTTP(res, req)
		case chi.RouteContext(req.Context()).RoutePattern() == "/user/export/opml":
			// Get all subscriptions.
			subscriptions, err := models.GetSubscriptions(req.Context(), a.Elastic)
			if err != nil {
				msg := models.NewErrorMessage(
					"Error exporting OPML file.",
					"The backend had issues generating the OPML file, please try again.",
				)
				renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
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
				msg := models.NewErrorMessage(
					"Error exporting OPML file.",
					"The backend had issues generating the OPML file, please try again.",
				)
				renderPartial(partials.ServerErrorNotification(msg)).ServeHTTP(res, req)
				return models.NewAPIError(err, http.StatusUnprocessableEntity)
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
	return alice.New().ThenFunc(handlerWithError(func(res http.ResponseWriter, req *http.Request) error {
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
				renderPartial(partials.AddCategory(req.URL.Path, category)).ServeHTTP(res, req)
			}
		case http.MethodDelete: // Remove a category.
			res.WriteHeader(http.StatusOK)
		default: // Unsupported, do nothing.
			res.WriteHeader(http.StatusNoContent)
		}
		return nil
	})).ServeHTTP
}

type (
	addSubscriptionRequests map[*models.SubscriptionRequest]*models.Feed
)

// feedURLs retrieves the URLs from the subscription requests.
func (r addSubscriptionRequests) feedURLs() []string {
	urls := make([]string, 0, len(r))
	for req := range maps.Keys(r) {
		urls = append(urls, req.URL)
	}
	return urls
}

// matchFeedsToSubscriptionRequests takes a list of subscription requests, extracts the URLs in each and attempt to
// match them to existing feeds. Where there is no existing feed, it will attempt to generate new feed data. It then
// stores the subscriptions that need new feeds and any with existing feeds in the context for the next handler.
func (r addSubscriptionRequests) matchFeedsToSubscriptionRequests(ctx context.Context, api *elastic.API) (layouts.AddSubscriptionResults, error) {
	// Extract user data.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Matching existing feeds to subscription requests...")

	// Paginate and gather all feeds matching the request URLs.
	var (
		feedPagination *models.Pagination
		existingFeeds  models.Feeds
	)
	for {
		count := 100
		feeds, nextResults, err := api.SearchFeeds(ctx, query.Terms("source_urls", r.feedURLs()...), count, nil, feedPagination)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}

		existingFeeds = append(existingFeeds, feeds...)

		if len(feeds) < count {
			break
		}
		feedPagination = &nextResults
	}

	results := make(layouts.AddSubscriptionResults)
	feedsNeeded := make(addSubscriptionRequests)

	// Loop over existing feeds.
	for request := range r {
		existingFeed := existingFeeds.FindByURL(request.GetURL())
		switch {
		case existingFeed == nil:
			// No existing feed, create a new one.
			newFeed, err := models.NewFeedFromURL(ctx, request.GetURL())
			if err != nil {
				msg := fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL())
				request.URLErr = errors.New(msg)
				results[request] = models.NewSubscriptionResult(nil, models.NewErrorMessage(msg, ""))
				continue
			}
			valid, err := validation.ValidateStruct(newFeed)
			if !valid || err != nil {
				msg := fmt.Sprintf("The feed URL %q cannot be parsed as a feed source or is not a valid URL.", request.GetURL())
				request.URLErr = errors.New(msg)
				results[request] = models.NewSubscriptionResult(nil, models.NewErrorMessage(msg, ""))
				continue
			}
			feedsNeeded[request] = newFeed
			slogctx.FromCtx(ctx).Debug("New feed needed for subscription.",
				slog.String("url", request.GetURL()),
				slog.String("feed", newFeed.GetTitle()),
			)
		case user.IsSubscribedToFeed(existingFeed.GetID()):
			// User already subscribed, ignore request.
			msg := "Already subscribed to " + existingFeed.GetTitle()
			request.URLErr = errors.New(msg)
			results[request] = models.NewSubscriptionResult(nil, models.NewWarningMessage(msg, ""))
		default:
			// Existing feed.
			r[request] = existingFeed
			slogctx.FromCtx(ctx).Debug("Existing feed for subscription.",
				slog.String("url", request.GetURL()),
				slog.String("feed", existingFeed.GetTitle()),
			)
		}
	}
	// Add new feeds for requests without an existing feed.
	if len(feedsNeeded) > 0 {
		newFeedsNeededResults, err := feedsNeeded.createNewFeeds(ctx, api)
		if err != nil {
			return nil, fmt.Errorf("matchFeedsToSubscriptions: %w", err)
		}
		maps.Copy(r, feedsNeeded)
		maps.Copy(results, newFeedsNeededResults)
	}

	return results, nil
}

func (r addSubscriptionRequests) createNewFeeds(ctx context.Context, api *elastic.API) (layouts.AddSubscriptionResults, error) {
	slogctx.FromCtx(ctx).Debug("Adding new feeds for subscriptions.")
	results := make(layouts.AddSubscriptionResults)

	// Testing no-op.
	// return results, nil

	// Add the new feeds.
	index, err := elastic.FeedsWriteIndexFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("createNewFeeds: %w", err)
	}
	addFeedsResults, err := elastic.BulkAdd(ctx, api, index, slices.Collect(maps.Values(r))...)
	if err != nil && !errors.Is(err, bulk.ErrBulkHasErrors) {
		return nil, fmt.Errorf("createNewFeeds: %w", err)
	}

	// Process the add feed results.
	for request, feed := range r {
		resp, found := addFeedsResults[feed.GetID()]
		if found {
			if resp.Created() {
				// Success, add request to map of subscription needed.
				r[request] = feed
			} else {
				results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
					Status:  models.UserMessageStatusError,
					Summary: "Internal Error. ",
					Details: "An internal, irrecoverable backend error occurred trying to add a subscription for the URL " + request.GetURL(),
				})
			}
		}
	}
	return results, nil
}

// AddSubscriptions handles adding new subscription via either the add or import user functionality. It
// handles: matching and filtering out requests against existing subscriptions, matching requests to existing feeds,
// creating new feeds as necessary and finally creating user subscriptions.
func (r addSubscriptionRequests) createNewSubscriptions(ctx context.Context, api *elastic.API) (layouts.AddSubscriptionResults, error) {
	// Extract user data.
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("createNewSubscriptions: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("Adding new subscriptions.")

	// Loop through the subscriptions adding their state to the existing subscription states slice. For any
	// subscriptions that have customisation data, collect the customisation data for adding later.
	results := make(layouts.AddSubscriptionResults)
	allMetadata := make(models.SubscriptionMetadataSlice, 0, len(r))
	for request, feed := range r {
		// // Ignore requests that have already got a message response, indicating some kind of failure or warning.
		// if r[request] != nil {
		// 	continue
		// }
		// Generate metadata and add to metadata slice.
		metadata := models.NewSubscriptionMetadata(user, feed, request)
		valid, err := metadata.Valid()
		if err != nil || !valid {
			slogctx.FromCtx(ctx).Debug("Invalid subscription metadata.",
				slog.Any("error", err),
				slog.String("feed_id", feed.GetID()),
				slog.String("feed", feed.GetTitle()),
				slog.String("url", request.GetURL()),
			)
			results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Subscription creation failed",
				Details: request.GetURL(),
			})
			continue
		}
		allMetadata = append(allMetadata, metadata)
		// Generate subscription and add to results map.
		subscription, err := models.GenerateSubscription(metadata, feed, 0, false)
		if err != nil {
			results[request] = models.NewSubscriptionResult(nil, &models.UserMessage{
				Status:  models.UserMessageStatusError,
				Summary: "Subscription creation failed",
				Details: request.GetURL(),
			})
			continue
		}
		results[request] = models.NewSubscriptionResult(subscription, &models.UserMessage{
			Status:  models.UserMessageStatusSuccess,
			Summary: "Subscription Created: " + feed.GetTitle(),
			Details: "Articles will be fetched shortly...",
		})
	}

	// Testing no-op.
	// return results, nil

	// Add the subscription states.
	if len(allMetadata) > 0 {
		user.AddSubscriptions(allMetadata...)
		// Disable onboarding once a subscription has been added.
		settings := user.GetSettings()
		if settings.ShowOnboarding {
			settings.ShowOnboarding = false
		}
		// Update the user object.
		err = api.UpdateUser(ctx, map[string]any{
			"subscriptions": user.Subscriptions,
			"settings":      settings,
		})
		if err != nil {
			return nil, fmt.Errorf("createNewSubscriptions: %w", err)
		}
	}
	return results, nil
}
