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
	"mime"
	"mime/multipart"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"
	"github.com/elastic/go-elasticsearch/v8/typedapi"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/pkg/opml"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

type OPMLFile struct {
	data multipart.File
	hdr  *multipart.FileHeader
}

func (f *OPMLFile) Load(data multipart.File, hdr *multipart.FileHeader) error {
	f.data = data
	f.hdr = hdr
	return nil
}

func (f *OPMLFile) Valid() bool {
	mediaType, _, err := mime.ParseMediaType(f.hdr.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mediaType == "text/x-opml+xml"
}

// NewSubscription creates a new APISubscription request and presents it as a
// form for the user to fill out.
func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.Modal(&api.SubscriptionRequest{})); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotImplemented)

	// ctx := req.Context()

	// user, found := models.UserFromCtx(ctx)
	// if !found {
	// 	logging.FromContext(ctx).Error("No user found?")
	// 	http.Error(res, "Problem!", http.StatusInternalServerError)

	// 	return
	// }

	// newSubscription, _, err := forms.DecodeForm[*api.SubscriptionRequest](req)
	// if err != nil {
	// 	logging.FromContext(ctx).Error("Could not decode submitted subscription request request.",
	// 		slog.Any("error", err))
	// 	http.Error(res, "Problem!", http.StatusInternalServerError)

	// 	return
	// }

	// addNewSubscription(req.Context(), s.DataAPI().GetAPI(), user, *newSubscription)

	// if err := htmx.NewResponse().Retarget("#command_modal").RenderTempl(ctx, res, subscription.AddSubscriptionSuccess()); err != nil {
	// 	logging.FromContext(ctx).Error("Cannot display content.",
	// 		slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
	// 	http.Error(res, "Problem!", http.StatusInternalServerError)
	// }
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
		spew.Dump(err)
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
			// data <- subscription.ImportError(&api.ExternalError{
			// 	Summary: "invalid input",
			// 	Details: "There is a problem with the inputs. Please check and try again.",
			// 	Err:     err,
			// })
			return
		}
		// Generate subscription requests using the import source.
		var requests api.SubscriptionRequests
		switch importMethod {
		case string(api.ImportSourceOPMLFile):
			// data <- subscription.ImportStatus("Processing OPML file...")
			data <- "Processing OPML file..."
			requests, err = processOPMLFileImport(req)
			if err != nil {
				// data <- subscription.ImportError(&api.ExternalError{
				// 	Summary: "failed processing OPML file",
				// 	Details: "The provided OPML file is invalid or contains problems. Please check the file and re-upload.",
				// 	Err:     err,
				// })
				return
			}
		}

		user, found := models.UserFromCtx(req.Context())
		if !found {
			logging.FromContext(req.Context()).Warn("Import processing failed.",
				slog.Any("error", err))
			// data <- subscription.ImportError(&api.ExternalError{
			// 	Summary: "invalid user data",
			// 	Details: "There was an internal (likely temporary) problem. Please try again.",
			// 	Err:     err,
			// })
			return
		}
		// data <- subscription.ImportStatus("Processing list of subscription requests...")
		data <- "Processing list of subscription requests..."

		// Process the requests.
		processRequests(req.Context(), data, s.DataAPI(), user, requests)
		for request := range slices.Values(requests) {

			if request.Err != nil {
				if errors.Is(request.Err, &api.ExternalError{}) {
					slog.Warn("request has error",
						slog.String("url", request.URL),
						slog.String("feed_id", request.FeedID),
						slog.Any("error", request.Err))
				} else {
					slog.Warn("request has error",
						slog.String("url", request.URL),
						slog.String("feed_id", request.FeedID),
						slog.Any("error", "unknown"))
				}
			} else {
				slog.Info("request ready",
					slog.String("url", request.URL),
					slog.String("feed_id", request.FeedID),
					slog.Bool("new_feed", request.Feed != nil))
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

func processOPMLFileImport(req *http.Request) (api.SubscriptionRequests, error) {
	// Decode the OPML file form input.
	opmlFile := &OPMLFile{}
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
	requests := make(api.SubscriptionRequests, 0, len(feeds))
	for _, feed := range feeds {
		requests = append(requests, &api.SubscriptionRequest{URL: feed.XMLURL})
	}

	return requests, nil
}

func (s Server) SaveSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ShowSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) RemoveSubscription(w http.ResponseWriter, r *http.Request, feedID models.FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (f *AddSubscriptionCategoryFormdataRequestBody) Valid() bool {
	return f.Category != ""
}

func (s Server) AddSubscriptionCategory(res http.ResponseWriter, req *http.Request) {
	var response templ.Component

	data, valid, err := forms.DecodeForm[*AddSubscriptionCategoryFormdataRequestBody](req)
	if !valid {
		response = subscription.AddSubscriptionWarning("Invalid ")
	} else {
		response = subscription.AddCategory(data.Category)
	}

	if err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, response); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) RemoveSubscriptionCategory(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)

	if _, err := res.Write(nil); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func addSubscriptionError(ctx context.Context, res http.ResponseWriter, details *api.SubscriptionRequest, message string) {
	var err error
	resp := htmx.NewResponse()

	if err = resp.RenderTempl(ctx, res, subscription.AddSubscriptionWarning(message)); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}

	if err = resp.RenderTempl(ctx, res, subscription.Form(details)); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func importError(ctx context.Context, res http.ResponseWriter, problem *api.ExternalError) {
	if err := htmx.NewResponse().Retarget(subscription.ImportFeedBackID.Target()).RenderTempl(ctx, res, subscription.ImportError(problem)); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func processRequests(ctx context.Context, data chan string, es *elastic.ElasticAPI, user *models.User, requests api.SubscriptionRequests) {
	// Add any new feeds required for subscriptions.
	// data <- subscription.ImportStatus("Gathering feed data...")
	data <- "Gathering feed data..."
	addSubscriptionFeeds(ctx, es, user, requests)
	// Add subscriptions.
	// data <- subscription.ImportStatus("Adding new subscriptions...")
	data <- "Adding new subscriptions..."
	addSubscriptions(ctx, es, requests)
}

// addSubscriptionFeeds will add new feeds for any subscriptions that require them.
func addSubscriptionFeeds(ctx context.Context, es *elastic.ElasticAPI, user *models.User, requests api.SubscriptionRequests) {
	// Generate feed information for each valid subscription request.
	generateFeedDetails(ctx, es.GetAPI(), user, requests.FilterValid())
	return
	// Add the new feeds.
	newFeedsResp, err := es.AddFeeds(ctx, requests.Feeds()...)
	// If the request failed and no new feeds were created, add a request error
	// indicating they are failed.
	if err != nil || (newFeedsResp.Err != nil && len(newFeedsResp.Responses) == 0) {
		for request := range slices.Values(requests.FilterFeedNeeded()) {
			request.Err = &api.ExternalError{
				Summary: "subscription could not be added",
				Details: "The subscription could not be added due to a temporary backend error. Please re-try.",
				Err:     err,
			}
		}
	}
	// Check the response results and add an error to each request that failed.
	for _, response := range newFeedsResp.Responses {
		if response.Id_ == nil {
			continue
		}
		_, err := response.State()
		if err != nil {
			if idx := slices.IndexFunc(requests.FilterFeedNeeded(), func(request *api.SubscriptionRequest) bool { return *response.Id_ == request.FeedID }); idx != -1 {
				requests[idx].Err = &api.ExternalError{
					Summary: "subscription could not be added",
					Details: "The subscription could not be added due to a temporary backend error. Please re-try.",
					Err:     err,
				}

			}
		}
	}
}

// generateFeedDetails adds details of the Feed that is associated with the
// subscription request. For non-existing feeds, the request will have a feed
// object added that can be used to add the feed as well.
func generateFeedDetails(ctx context.Context, esapi *typedapi.API, user *models.User, requests api.SubscriptionRequests) {
	// Collect existing feeds matching the URls.
	existingFeeds, err := elastic.GetFeedsByURL(ctx, esapi, requests.URLs()...)
	if err != nil {
		for request := range slices.Values(requests) {
			request.Err = &api.ExternalError{
				Summary: fmt.Sprintf("could not create a subscription for feed URL %s", request.GetURL()),
				Details: "A subscription could not be created as temporary backend error. Please check the URL and/or try again.",
				Err:     err,
			}
		}
	}
	// Loop through requests and generate subscriptions for each one. If a new
	// feed needs to be added, also create those.
	for request := range slices.Values(requests) {
		if idx := slices.IndexFunc(existingFeeds, func(feed models.APIFeed) bool {
			return request.GetURL() == feed.GetLink()
		}); idx != -1 {
			// Existing feed. Add existing FeedID to request.
			request.FeedID = existingFeeds[idx].GetID()
		} else {
			// New feed. Create feed and add FeedID and Feed to request.
			newFeed, err := models.NewFeedFromURL(ctx, request.GetURL())
			if err != nil {
				request.Err = &api.ExternalError{
					Summary: fmt.Sprintf("could not create a subscription for feed URL %s", request.GetURL()),
					Details: "A subscription could not be created as there was an error trying to parse the feed at the given URL. Please check the URL and/or try again.",
					Err:     err,
				}
			} else {
				request.FeedID = newFeed.ID
				request.Feed = newFeed
			}
		}
	}
}

// addSubscriptions will add new subscriptions for all valid subscription requests.
func addSubscriptions(ctx context.Context, es *elastic.ElasticAPI, requests api.SubscriptionRequests) {
	subscriptions := generateSubscriptions(requests.FilterValid())
	return
	// Add the new subscriptions.
	newSubsResp, err := es.AddSubscriptions(ctx, subscriptions...)
	// If the request failed and no new feeds were created, add a request error
	// indicating they are failed.
	if err != nil || (newSubsResp.Err != nil && len(newSubsResp.Responses) == 0) {
		for request := range slices.Values(requests) {
			request.Err = &api.ExternalError{
				Summary: "subscription could not be added",
				Details: "The subscription could not be added due to a temporary backend error. Please re-try.",
				Err:     err,
			}
		}
	}
	// Check the response results and add an error to each request that failed.
	for _, response := range newSubsResp.Responses {
		if response.Id_ == nil {
			continue
		}
		_, err := response.State()
		if err != nil {
			if idx := slices.IndexFunc(requests, func(request *api.SubscriptionRequest) bool { return *response.Id_ == request.FeedID }); idx != -1 {
				requests[idx].Err = &api.ExternalError{
					Summary: "subscription could not be added",
					Details: "The subscription could not be added due to a temporary backend error. Please re-try.",
					Err:     err,
				}

			}
		}
	}
}

// generateSubscriptions will generate subscription objects for all valid subscription requests.
func generateSubscriptions(requests api.SubscriptionRequests) []*models.Subscription {
	subscriptions := make([]*models.Subscription, 0, len(requests))
	for request := range slices.Values(requests) {
		subscription, err := request.ToSubscription()
		if err != nil {
			request.Err = &api.ExternalError{
				Summary: "invalid subscription data",
				Details: "The subscription contains invalid data. Please check the input and try again.",
				Err:     err,
			}
		} else {
			subscriptions = append(subscriptions, subscription)
		}
	}
	return subscriptions
}
