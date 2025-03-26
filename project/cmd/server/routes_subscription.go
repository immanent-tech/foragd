// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/cmd/server/handlers"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/bulk"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

// subscriptionRequests maps a Subscription to a Feed that needs to be created
// for the Subscription.
type subscriptionRequests map[*models.Subscription]*models.Feed

// subscriptionRequestResults maps a SubscriptionID for a SubscriptionRequest to
// message indicating the result of processing the SubscriptionRequest.
type subscriptionRequestResults map[*models.Subscription]*models.Message

func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().
		RenderTempl(req.Context(), res,
			subscription.NewSubscriptionModal(models.NewSubscriptionRequest(""), nil)); err != nil {
		handlers.InternalServerError(res, req, err)
		return
	}
}

func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
	if err != nil {
		msg := models.NewMessage("Error parsing request.",
			models.WithDetails("The request could not be parsed. This is likely a temporary problem. Please try again."),
			models.WithError(err),
		)
		showRequestResponse(res, req, models.NewSubscriptionRequest(""), msg)
		return
	}
	if !valid {
		msg := models.NewMessage("Invalid request data.",
			models.WithDetails("The request contains invalid data. Please check and try again."),
			models.WithError(err),
		)
		showRequestResponse(res, req, request, msg)
		return
	}

	results := processSubscriptionRequests(req.Context(), s.DataAPI(), models.SubscriptionRequests{request})
	for msg := range maps.Values(results) {
		if err := htmx.NewResponse().
			Retarget(subscription.SubscriptionModalID.Target()).
			Reswap(htmx.SwapOuterHTML).
			RenderTempl(req.Context(), res, subscription.NewSubscriptionResultsModal(msg)); err != nil {
			handlers.InternalServerError(res, req, err)
			return
		}
		spew.Dump(msg)
	}
}

func (s Server) StartImport(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.ImportModal()); err != nil {
		handlers.InternalServerError(res, req, err)
		return
	}
}

func (f *SetImportMethodFormdataBody) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false, err
	}
	return true, nil
}

func (s Server) SetImportMethod(res http.ResponseWriter, req *http.Request) {
	importMethod, valid, err := forms.DecodeForm[*SetImportMethodFormdataBody](req)
	if err != nil || !valid {
		msg := models.NewMessage("Error setting up import.",
			models.WithDetails("There was an error setting up the import. Please try again."),
			models.WithStatus(models.MessageStatusError),
			models.WithError(err))
		showImportFailed(res, req, msg)
		return
	}

	var form templ.Component
	switch importMethod.From {
	case "opml_file":
		form = subscription.ImportFromOPML()
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, form); err != nil {
		handlers.InternalServerError(res, req, err)
		return
	}
}

func (s Server) ProcessImport(res http.ResponseWriter, req *http.Request) {
	var requests models.SubscriptionRequests
	var results subscriptionRequestResults
	// Decode the import source.
	importMethod, err := forms.DecodeMultipartValue(req, "source")
	if err != nil {
		msg := models.NewMessage("Error reading import source data.",
			models.WithDetails("There was an error setting up the import. Please try again."),
			models.WithStatus(models.MessageStatusError),
			models.WithError(err))
		showImportFailed(res, req, msg)
		return
	}
	// Generate subscription requests using the import source.
	switch importMethod {
	case string(models.ImportSourceOPMLFile):
		requests, err = processOPMLFileImport(req)
		if err != nil {
			msg := models.NewMessage("Error processing OPML file.",
				models.WithDetails("There was an error processing the OPML file. Please try again."),
				models.WithStatus(models.MessageStatusError),
				models.WithError(err))
			showImportFailed(res, req, msg)
			return
		}
	}
	// Process the requests.
	results = processSubscriptionRequests(req.Context(), s.DataAPI(), requests)
	// Show results.
	showImportResults(res, req, results)
}

func processOPMLFileImport(req *http.Request) (models.SubscriptionRequests, error) {
	// Decode the OPML file form input.
	opmlFile := &models.OPMLFile{}
	opmlFile, valid, err := forms.DecodeMultipartFile(req, "data", opmlFile)
	if err != nil || !valid {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}
	opmlImport, err := opmlFile.Parse()
	if err != nil {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}
	// Extract the individual feeds from the OPML object and create a subscription
	// request for each one.
	feeds := opmlImport.ExtractRSS()
	requests := make(models.SubscriptionRequests, 0, len(feeds))
	for _, feed := range feeds {
		requests = append(requests, models.NewSubscriptionRequest(feed.XMLURL))
	}

	return requests, nil
}

func (s Server) EditSubscription(res http.ResponseWriter, req *http.Request, subscription SubscriptionID) {
	res.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ShowSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) SaveSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) RemoveSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (f *AddSubscriptionCategoryFormdataBody) Valid() (bool, error) {
	if f.Category == "" {
		return false, errors.New("invalid empty category")
	}
	return true, nil
}

func (s Server) AddSubscriptionCategory(res http.ResponseWriter, req *http.Request) {
	data, valid, err := forms.DecodeForm[*AddSubscriptionCategoryFormdataBody](req)
	if !valid || err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.ShowCategory(data.Category)); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) DelSubscriptionCategory(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)

	if _, err := res.Write(nil); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

// processSubscriptionRequests is the main action loop for processing
// subscription requests. It handles generating the subscriptions, adding any
// new feeds that are required and updating the user object. As it processes
// requests, results are gathered. It will return a map of results.
func processSubscriptionRequests(ctx context.Context, api DataAPI, requests models.SubscriptionRequests) subscriptionRequestResults {
	results := make(subscriptionRequestResults)
	// Generate subscriptions.
	subscriptions, warnings := generateSubscriptions(ctx, api, requests...)
	maps.Copy(results, warnings)
	for sub := range maps.Keys(subscriptions) {
		results[sub] = models.NewMessage(
			fmt.Sprintf("Subscription for feed %s created!", sub.GetName()),
			models.WithStatus(models.MessageStatusSuccess))
	}
	return results
	// Add any new feeds required.
	warnings = addFeedsForSubscriptions(ctx, api, filterFeedNeeded(subscriptions))
	maps.Copy(results, warnings)
	// Filter subscriptions that have failed results.
	validSubscriptions := maps.Collect(models.FilterMap(subscriptions, func(s *models.Subscription, _ *models.Feed) bool {
		return !slices.ContainsFunc(slices.Collect(maps.Keys(results)), func(v *models.Subscription) bool { return v.ID == s.ID })
	}))
	err := api.AddSubscriptions(ctx, slices.Collect(maps.Keys(validSubscriptions)))
	// If the request to add subscriptions failed, record failure for all subscriptions.
	for sub := range maps.Keys(validSubscriptions) {
		if err != nil {
			results[sub] = models.NewMessage(
				fmt.Sprintf("could not create a subscription for feed %s", sub.GetName()),
				models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
				models.WithStatus(models.MessageStatusWarning),
				models.WithError(err),
			)
		} else {
			results[sub] = models.NewMessage(
				fmt.Sprintf("Subscription for feed %s created!", sub.GetName()),
				models.WithStatus(models.MessageStatusSuccess))
		}
	}
	return results
}

// addFeedsForSubscriptions will take a map of Subscriptions needing Feeds and
// try to create new Feed objects for them. It returns a map of the
// SubscriptionID and either a nil value (feed created) or non-nil Message
// (error occurred).
func addFeedsForSubscriptions(ctx context.Context, api DataAPI, subscriptions map[*models.Subscription]*models.Feed) subscriptionRequestResults {
	// No-op if there are no Subscriptions needing Feeds.
	if len(subscriptions) == 0 {
		return nil
	}
	results := make(subscriptionRequestResults)
	// Add the new feeds.
	newFeedsResp, err := api.AddFeeds(ctx, slices.Collect(maps.Values(subscriptions))...)
	// If the request failed and no new feeds were created return all
	// subscriptions with fail messages.
	if err != nil || (newFeedsResp.Err != nil && len(newFeedsResp.Responses) == 0) {
		for sub := range maps.Keys(subscriptions) {
			results[sub] = models.NewMessage(
				fmt.Sprintf("could not create a subscription for feed %s", sub.GetName()),
				models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
				models.WithStatus(models.MessageStatusWarning),
				models.WithError(err),
			)
		}
		return results
	}
	// Else loop through the results, adding fail messages for any subscriptions
	// that failed to have their feed created.
	for sub := range maps.Keys(subscriptions) {
		// Match a response result to a subscription.
		idx := slices.IndexFunc(newFeedsResp.Responses,
			func(v *bulk.OperationResponse) bool {
				if v.Id_ != nil {
					return *v.Id_ == sub.ID
				}
				return false
			})
		if idx != -1 {
			// For a matched response, check if the response indicates an error.
			// If it does, add a message.
			if _, err = newFeedsResp.Responses[idx].State(); err != nil {
				results[sub] = models.NewMessage(
					fmt.Sprintf("could not create a subscription for feed %s", sub.GetName()),
					models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
					models.WithStatus(models.MessageStatusWarning),
					models.WithError(err),
				)
			}
			results[sub] = nil
		} else {
			results[sub] = models.NewMessage(
				fmt.Sprintf("could not create a subscription for feed %s", sub.GetName()),
				models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
				models.WithStatus(models.MessageStatusWarning),
				models.WithError(err),
			)
		}
	}

	return results
}

// generateSubscriptions adds details of the Feed that is associated with the
// subscription request. For non-existing feeds, the request will have a feed
// object added that can be used to add the feed as well.
func generateSubscriptions(ctx context.Context, api DataAPI, requests ...*models.SubscriptionRequest) (subscriptionRequests, subscriptionRequestResults) {
	newSubscriptions := make(subscriptionRequests)
	results := make(subscriptionRequestResults)

	// Collect existing feeds matching the URls.
	existingFeeds, err := api.GetFeedsByURL(ctx, models.SubscriptionRequests(requests).URLs()...)
	if err != nil {
		for request := range slices.Values(requests) {
			results[&models.Subscription{ID: request.ID}] = models.NewMessage(
				fmt.Sprintf("could not create a subscription for feed with URL %s", request.GetURL()),
				models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
				models.WithStatus(models.MessageStatusWarning),
				models.WithError(err),
			)
		}
		return nil, results
	}
	// Loop through requests and generate subscriptions for each one. If a new
	// feed needs to be added, also create those.
	for request := range slices.Values(requests) {
		if idx := slices.IndexFunc(existingFeeds, func(feed *models.APIFeed) bool {
			return request.GetURL() == feed.GetLink()
		}); idx != -1 {
			// Existing Ffeed. Create Subscription using existing Feed details.
			s := models.NewSubscription(request, existingFeeds[idx])
			newSubscriptions[s] = nil
		} else {
			// New Feed. Create Feed then create Subscription with new Feed details.
			newFeed, err := models.NewFeedFromURL(ctx, request.GetURL())
			if err != nil {
				results[&models.Subscription{ID: request.ID}] = models.NewMessage(fmt.Sprintf("could not create a subscription for feed with URL %s", request.GetURL()),
					models.WithDetails("A subscription could not be created as there was an error trying to parse the feed at the given URL. Please check the URL and/or try again."),
					models.WithStatus(models.MessageStatusWarning),
					models.WithError(err),
				)
			} else {
				s := models.NewSubscription(request, newFeed)
				newSubscriptions[s] = newFeed
			}
		}
	}
	return newSubscriptions, results
}

func filterFeedNeeded(subscriptions map[*models.Subscription]*models.Feed) map[*models.Subscription]*models.Feed {
	return maps.Collect(models.FilterMap(subscriptions, func(s *models.Subscription, f *models.Feed) bool {
		spew.Dump(s.ID, f != nil)
		return f != nil
	}))
}

func showRequestResponse(res http.ResponseWriter, req *http.Request, request *models.SubscriptionRequest, msg *models.Message) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.NewSubscriptionRequest(request).Form(msg)); err != nil {
		handlers.InternalServerError(res, req, err)
	}
}

func showImportFailed(res http.ResponseWriter, req *http.Request, msg *models.Message) {
	if err := htmx.NewResponse().
		Retarget(subscription.ImportModalID.Target()).
		Reswap(htmx.SwapOuterHTML).
		RenderTempl(req.Context(), res,
			subscription.ImportResultsModal(subscription.ImportFailed(msg)),
		); err != nil {
		handlers.InternalServerError(res, req, err)
	}
}

func showImportResults(res http.ResponseWriter, req *http.Request, results subscriptionRequestResults) {
	failed := maps.Collect(models.FilterMap(results, func(_ *models.Subscription, v *models.Message) bool {
		return v.Status != models.MessageStatusSuccess
	}))
	successful := maps.Collect(models.FilterMap(results, func(_ *models.Subscription, v *models.Message) bool {
		return v.Status == models.MessageStatusSuccess
	}))

	var resultsFile strings.Builder
	fmt.Fprintf(&resultsFile, `<script id="resultscsv" type="text/csv">`)
	fmt.Fprintf(&resultsFile, "feed,status,summary,details\n")
	for k, v := range maps.All(results) {
		var name, details string
		if k.GetName() != "" {
			name = k.GetName()
		} else {
			name = k.FeedDetails.URL
		}
		if v.Details != nil {
			details = *v.Details
		}
		fmt.Fprintf(&resultsFile, `"%s",%s,"%s","%s"`+"\n", name, v.Status, v.Summary, details)
	}
	fmt.Fprintf(&resultsFile, "</script>")

	if err := htmx.NewResponse().
		Retarget(subscription.ImportModalID.Target()).
		Reswap(htmx.SwapOuterHTML).
		RenderTempl(req.Context(), res,
			subscription.ImportResultsModal(subscription.ImportSuccess(len(successful), len(failed), resultsFile.String())),
		); err != nil {
		handlers.InternalServerError(res, req, err)
	}
}
