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

	"github.com/joshuar/go-feed-me/internal/app/server/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic"
	"github.com/joshuar/go-feed-me/internal/platforms/elastic/schema"
	"github.com/joshuar/go-feed-me/web/templates/partials/subscription"
)

func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	id, subscriptionRequest, err := models.NewSubscriptionRequest()
	if err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.", slog.Any("error", err))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, subscription.Modal(id, subscriptionRequest)); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) SaveSubscription(res http.ResponseWriter, req *http.Request, subscriptionID SubscriptionID) {
	ctx := req.Context()
	ctx = elastic.FeedsIndexToCtx(ctx, schema.FeedsSchemaPrefix)
	ctx = elastic.UserIndexToCtx(ctx, schema.UsersSchemaPrefix)

	newSubscription, valid, err := forms.DecodeForm[*models.APISubscriptionRequest](req)
	if err != nil {
		logging.FromContext(ctx).Error("Could not decode submitted subscription request request.",
			slog.Any("error", err))
		return
	}

	newSubscription.SubscriptionID = subscriptionID

	if !valid {
		var err error
		resp := htmx.NewResponse()

		if err = resp.RenderTempl(ctx, res, subscription.AddSubscriptionWarning()); err != nil {
			logging.FromContext(ctx).Error("Cannot display content.",
				slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err = resp.RenderTempl(ctx, res, subscription.Form(newSubscription.SubscriptionID, newSubscription)); err != nil {
			logging.FromContext(ctx).Error("Cannot display content.",
				slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		return
	}

	err = s.API.elastic.AddSubscriptions(ctx, newSubscription)
	if err != nil {
		var err error

		logging.FromContext(ctx).Error("Could not add subscription.",
			slog.Any("error", err))

		if newSubscription.ValidationErrors == nil {
			newSubscription.ValidationErrors = make(map[string]string)
		}

		newSubscription.ValidationErrors["URL"] = "Unable to verify URL maps to a feed."

		resp := htmx.NewResponse()

		if err = resp.RenderTempl(ctx, res, subscription.Form(subscriptionID, newSubscription)); err != nil {
			logging.FromContext(ctx).Error("Cannot display content.",
				slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		if err = resp.RenderTempl(ctx, res, subscription.AddSubscriptionWarning()); err != nil {
			logging.FromContext(ctx).Error("Cannot display content.",
				slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		return
	}

	if err := htmx.NewResponse().Retarget("#command_modal").RenderTempl(ctx, res, subscription.AddSubscriptionSuccess()); err != nil {
		logging.FromContext(ctx).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) ShowSubscription(w http.ResponseWriter, r *http.Request, subID SubscriptionID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s Server) RemoveSubscription(w http.ResponseWriter, r *http.Request, subID SubscriptionID) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (f *AddSubscriptionCategoryFormdataRequestBody) Valid() bool {
	return f.Category != ""
}

func (s Server) AddSubscriptionCategory(res http.ResponseWriter, req *http.Request, subID SubscriptionID) {
	var response templ.Component

	data, valid, err := forms.DecodeForm[*AddSubscriptionCategoryFormdataRequestBody](req)
	if !valid {
		response = subscription.AddSubscriptionWarning()
	} else {
		response = subscription.AddCategory(subID, data.Category)
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

func (s Server) RemoveSubscriptionCategory(res http.ResponseWriter, req *http.Request, _ SubscriptionID) {
	res.WriteHeader(http.StatusOK)

	if _, err := res.Write(nil); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}
