// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"mime"
	"mime/multipart"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/bulk"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/pkg/opml"
	"github.com/joshuar/go-feed-me/web/templates/partials"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

type opmlFile struct {
	data multipart.File
	hdr  *multipart.FileHeader
}

func (f *opmlFile) Load(data multipart.File, hdr *multipart.FileHeader) error {
	f.data = data
	f.hdr = hdr
	return nil
}

func (f *opmlFile) Valid() bool {
	mediaType, _, err := mime.ParseMediaType(f.hdr.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mediaType == "text/x-opml+xml"
}

// subscriptionRequests maps a Subscription to a Feed that needs to be created
// for the Subscription.
type subscriptionRequests map[*models.Subscription]*models.Feed

// subscriptionRequestResults maps a SubscriptionID for a SubscriptionRequest to
// message indicating the result of processing the SubscriptionRequest.
type subscriptionRequestResults map[models.SubscriptionID]*models.Message

func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.Modal(models.NewSubscriptionRequest())); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	request, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
	if !valid || err != nil {
		if err := htmx.NewResponse().Retarget("#warnings_toast").RenderTempl(req.Context(), res, partials.NewUserMessage(request.Msg).Show()); err != nil {
			logging.FromContext(req.Context()).Warn("Bad request.", slog.Any("error", err))
			http.Error(res, "user signup failed!", http.StatusInternalServerError)
		}
	}

	results := processSubscriptionRequests(req.Context(), s.DataAPI(), models.SubscriptionRequests{request})
	spew.Dump(request, results)
}

func (s Server) StartImport(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.ImportModal()); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}

	// w.WriteHeader(http.StatusNotImplemented)
}

func (f *SetImportMethodFormdataBody) Valid() bool {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false
	}
	return true
}

func (s Server) SetImportMethod(res http.ResponseWriter, req *http.Request) {
	importMethod, valid, err := forms.DecodeForm[*SetImportMethodFormdataBody](req)
	switch {
	case err != nil:
		slog.Error("error parsing form", slog.Any("error", err))
	case !valid:
		slog.Error("invalid input")
	}

	var form templ.Component
	switch importMethod.From {
	case "opml_file":
		form = subscription.ImportFromOPML()
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, form); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) ProcessImport(res http.ResponseWriter, req *http.Request) {
	// data := make(chan templ.Component)
	data := make(chan string)

	// Serve using the streaming mode of the handler.
	go func() {
		defer close(data)
		// data <- subscription.ImportStatus("Decoding form data...")
		data <- "Decoding form data..."
		// Decode the import source.
		importMethod, err := forms.DecodeMultipartValue(req, "source")
		if err != nil {
			logging.FromContext(req.Context()).Warn("Import processing failed.",
				slog.Any("error", err))
			// data <- subscription.ImportError(&models.ExternalError{
			// 	Summary: "invalid input",
			// 	Details: "There is a problem with the inputs. Please check and try again.",
			// 	Err:     err,
			// })
			return
		}
		// Generate subscription requests using the import source.
		var requests models.SubscriptionRequests
		switch importMethod {
		case string(models.ImportSourceOPMLFile):
			// data <- subscription.ImportStatus("Processing OPML file...")
			data <- "Processing OPML file..."
			requests, err = processOPMLFileImport(req)
			if err != nil {
				// data <- subscription.ImportError(&models.ExternalError{
				// 	Summary: "failed processing OPML file",
				// 	Details: "The provided OPML file is invalid or contains problems. Please check the file and re-upload.",
				// 	Err:     err,
				// })
				return
			}
		}

		// data <- subscription.ImportStatus("Processing list of subscription requests...")
		data <- "Processing list of subscription requests..."

		// Process the requests.
		processSubscriptionRequests(req.Context(), s.DataAPI(), requests)
		for request := range slices.Values(requests) {

			if request.Msg != nil {
				if errors.Is(request.Msg, &models.Message{}) {
					slog.Warn("request has error",
						slog.String("url", request.URL),
						slog.Any("error", request.Msg.Error()))
				} else {
					slog.Warn("request has error",
						slog.String("url", request.URL),
						slog.Any("error", "unknown"))
				}
			} else {
				slog.Info("request ready",
					slog.String("url", request.URL))
			}
		}
		logging.FromContext(req.Context()).Debug("Import finished")
	}()

	resp := htmx.NewResponse()
	resp.Write(res)

	// templ.Handler(subscription.ImportProcessing(data),
	// templ.WithStreaming()).ServeHTTP(res, req)

	if err := resp.RenderTempl(req.Context(), res, subscription.ImportProcessing(data)); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
	logging.FromContext(req.Context()).Debug("Stopped processing")

	// spew.Dump(requests)
}

func processOPMLFileImport(req *http.Request) (models.SubscriptionRequests, error) {
	// Decode the OPML file form input.
	opmlFile := &opmlFile{}
	opmlFile, valid, err := forms.DecodeMultipartFile(req, "data", opmlFile)
	if err != nil || !valid {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}
	// Read the OPML file data into a byte array.
	data, err := io.ReadAll(opmlFile.data)
	if err != nil {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}
	// Parse and create an OPML object from the byte array.
	opmlImport, err := opml.New(data)
	if err != nil {
		return nil, fmt.Errorf("decode OPML file data failed: %w", err)
	}
	// Extract the individual feeds from the OPML object and create a subscription
	// request for each one.
	feeds := opmlImport.ExtractRSS()
	requests := make(models.SubscriptionRequests, 0, len(feeds))
	for _, feed := range feeds {
		requests = append(requests, &models.SubscriptionRequest{URL: feed.XMLURL})
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

func (f *AddSubscriptionCategoryFormdataBody) Valid() bool {
	return f.Category != ""
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
	maps.Insert(results, maps.All(warnings))
	// Add any new feeds required.
	warnings = addFeedsForSubscriptions(ctx, api, filterFeedNeeded(subscriptions))
	maps.Insert(results, maps.All(warnings))
	// Filter subscriptions that have failed results.
	subscriptions = maps.Collect(models.FilterMap(subscriptions, func(s *models.Subscription, _ *models.Feed) bool {
		return !slices.ContainsFunc(slices.Collect(maps.Keys(results)), func(i models.SubscriptionID) bool { return i == s.ID })
	}))
	err := api.AddSubscriptions(ctx, slices.Collect(maps.Keys(subscriptions)))
	// If the request to add subscriptions failed, record failure for all subscriptions.
	for sub := range maps.Keys(subscriptions) {
		if err != nil {
			results[sub.ID] = models.NewMessage(
				fmt.Sprintf("could not create a subscription for feed %s", sub.GetName()),
				models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
				models.WithStatus(models.MessageStatusWarning),
				models.WithError(err),
			)
		} else {
			results[sub.ID] = models.NewMessage(
				fmt.Sprintf("Subscription for feed %s created!", sub.GetName()),
				models.WithStatus(models.MessageStatusSuccess),
				models.WithError(err))
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
			results[sub.ID] = models.NewMessage(
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
				results[sub.ID] = models.NewMessage(
					fmt.Sprintf("could not create a subscription for feed %s", sub.GetName()),
					models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
					models.WithStatus(models.MessageStatusWarning),
					models.WithError(err),
				)
			}
			results[sub.ID] = nil
		} else {
			results[sub.ID] = models.NewMessage(
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
			results[request.ID] = models.NewMessage(
				fmt.Sprintf("could not create a subscription for feed with URL %s", request.GetURL()),
				models.WithDetails("A subscription could not be created as temporary backend error. Please check the URL and/or try again."),
				models.WithStatus(models.MessageStatusWarning),
				models.WithError(err),
			)
		}
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
				results[request.ID] = models.NewMessage(fmt.Sprintf("could not create a subscription for feed with URL %s", request.GetURL()),
					models.WithDetails("A subscription could not be created as there was an error trying to parse the feed at the given URL. Please check the URL and/or try again."),
					models.WithStatus(models.MessageStatusWarning),
					models.WithError(err),
				)
			} else {
				s := models.NewSubscription(request, existingFeeds[idx])
				newSubscriptions[s] = newFeed
			}
		}
	}
	return newSubscriptions, results
}

func filterFeedNeeded(subscriptions map[*models.Subscription]*models.Feed) map[*models.Subscription]*models.Feed {
	return maps.Collect(models.FilterMap(subscriptions, func(_ *models.Subscription, f *models.Feed) bool {
		return f != nil
	}))
}
