// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	slogctx "github.com/veqryn/slog-context"

	"github.com/joshuar/go-feed-me/cmd/server/handlers"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/validation"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	handler := handlers.HTMXResponse(htmx.NewResponse(), subscription.NewSubscriptionModal(models.NewSubscriptionRequest(""), nil))
	handler.ServeHTTP(res, req)
}

// AddSubscription handles an add subscription request.
func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	ctx := slogctx.With(req.Context(), slog.String("id", middleware.GetReqID(req.Context())))
	ctx = slogctx.WithGroup(ctx, "add_subscription")
	chain := alice.New(
		handlers.ParseSubscriptionRequest,
		handlers.MatchRequestsWithFeeds(s.DataAPI()),
		handlers.CreateNewFeedsForRequests,
		handlers.AddFeedsForRequests(s.DataAPI()),
		handlers.AddSubscriptionsForRequests(s.DataAPI()),
	).Then(handlers.AddSubscriptionResults())
	chain.ServeHTTP(res, req.WithContext(ctx))
}

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

func (s Server) ProcessImport(res http.ResponseWriter, req *http.Request) {
	ctx := slogctx.With(req.Context(), slog.String("route", "import_subscriptions"))
	ctx = slogctx.With(ctx, slog.String("id", middleware.GetReqID(req.Context())))
	chain := alice.New(
		handlers.ProcessImportMethod,
		handlers.MatchRequestsWithFeeds(s.DataAPI()),
		handlers.CreateNewFeedsForRequests,
		handlers.AddFeedsForRequests(s.DataAPI()),
		handlers.AddSubscriptionsForRequests(s.DataAPI()),
	).Then(handlers.ImportResults(nil))
	chain.ServeHTTP(res, req.WithContext(ctx))
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
		slogctx.FromCtx(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.ShowCategory(data.Category)); err != nil {
		slogctx.FromCtx(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) DelSubscriptionCategory(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)

	if _, err := res.Write(nil); err != nil {
		slogctx.FromCtx(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
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
