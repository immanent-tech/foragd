// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/justinas/alice"

	"github.com/joshuar/go-feed-me/components/validation"
	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/server/forms"
	"github.com/joshuar/go-feed-me/server/handlers"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	handler := handlers.HTMXResponse(htmx.NewResponse(), subscription.NewSubscriptionModal(models.NewSubscriptionRequest(""), nil))
	handler.ServeHTTP(res, req)
}

// AddSubscription handles an add subscription request.
func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger("add_subscription"),
		handlers.ParseSubscriptionRequest,
		handlers.MatchRequestsWithFeeds(s.DataAPI()),
		handlers.CreateNewFeedsForRequests,
		handlers.AddFeedsForRequests(s.DataAPI()),
		handlers.AddSubscriptionsForRequests(s.DataAPI()),
	).Then(handlers.AddSubscriptionResults())
	chain.ServeHTTP(res, req)
}

// StartImport sets up an import for the user.
func (s Server) StartImport(res http.ResponseWriter, req *http.Request) {
	handler := handlers.HTMXResponse(htmx.NewResponse(), subscription.ImportModal())
	handler.ServeHTTP(res, req)
}

func (f *SetImportMethodFormdataBody) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(f)
	if !valid || err != nil {
		return false, err
	}
	return true, nil
}

// SetImportMethod parses the selected import method and calls the appropriate handler to handle that import method.
func (s Server) SetImportMethod(res http.ResponseWriter, req *http.Request) {
	importMethod, valid, err := forms.DecodeForm[*SetImportMethodFormdataBody](req)
	if err != nil || !valid {
		msg := models.NewMessage(
			"Error processing import.",
			models.MessageStatusError,
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

// ProcessImport performs the actions required to import requests from any source.
func (s Server) ProcessImport(res http.ResponseWriter, req *http.Request) {
	chain := alice.New(
		handlers.RouteLogger("import_subscriptions"),
		handlers.ProcessImportMethod,
		handlers.MatchRequestsWithFeeds(s.DataAPI()),
		handlers.CreateNewFeedsForRequests,
		handlers.AddFeedsForRequests(s.DataAPI()),
		handlers.AddSubscriptionsForRequests(s.DataAPI()),
	).Then(handlers.ImportResults(nil))
	chain.ServeHTTP(res, req)
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

func (s Server) RemoveSubscription(res http.ResponseWriter, req *http.Request, subscriptionID models.SubscriptionID, params RemoveSubscriptionParams) {
	chain := alice.New(
		handlers.RouteLogger("remove_subscriptions"),
	).Then(handlers.RemoveSubscription(s.DataAPI(), subscriptionID, params.Decision))
	chain.ServeHTTP(res, req)
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
