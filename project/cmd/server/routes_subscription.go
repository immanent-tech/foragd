// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"

	"github.com/joshuar/go-feed-me/internal/api"
	"github.com/joshuar/go-feed-me/internal/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

// NewSubscription creates a new APISubscription request and presents it as a
// form for the user to fill out.
func (s Server) NewSubscription(res http.ResponseWriter, req *http.Request) {
	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.Modal(&api.APISubscriptionRequest{})); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	user, found := models.UserFromCtx(ctx)
	if !found {
		logging.FromContext(ctx).Error("No user found?")
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}

	newSubscription, valid, err := forms.DecodeForm[*api.APISubscriptionRequest](req)
	if err != nil {
		logging.FromContext(ctx).Error("Could not decode submitted subscription request request.",
			slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)

		return
	}

	if !valid {
		addSubscriptionError(ctx, res, newSubscription, "Please check your inputs and try again.")
		return
	}

	// Find any existing feed with the subscription URL or create a new feed.
	feedID, err := models.FindOrAddFeed(ctx, s.API.elastic, newSubscription.URL)
	if err != nil {
		addSubscriptionError(ctx, res, newSubscription, "Unable to verify URL maps to a feed.")
		return
	}

	err = elastic.UserActionAddSubscription(ctx, user, s.API.elastic, feedID, newSubscription)
	if err != nil {
		addSubscriptionError(ctx, res, newSubscription, "There was a temporary problem adding a subscription. Please try again.")
		return
	}

	if err := htmx.NewResponse().Retarget("#command_modal").RenderTempl(ctx, res, subscription.AddSubscriptionSuccess()); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) SaveSubscription(w http.ResponseWriter, r *http.Request, feedID FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) ShowSubscription(w http.ResponseWriter, r *http.Request, feedID FeedID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) RemoveSubscription(w http.ResponseWriter, r *http.Request, feedID FeedID) {
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

func addSubscriptionError(ctx context.Context, res http.ResponseWriter, details *api.APISubscriptionRequest, message string) {
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
