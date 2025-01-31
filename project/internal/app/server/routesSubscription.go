// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:get-return
package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/angelofallars/htmx-go"
	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/app/server/forms"
	"github.com/joshuar/go-feed-me/internal/logging"
	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/web/templates/partials/content"
)

func (s Server) AddSubscription(res http.ResponseWriter, req *http.Request) {
	ctx := models.SubscriptionRequestToCtx(req.Context(), models.NewSubscriptionAddRequest())

	if err := htmx.NewResponse().RenderTempl(ctx, res, content.AddSubscriptionForm()); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

func (s Server) ProcessAddSubscription(res http.ResponseWriter, req *http.Request) {
	subscription, valid, err := forms.DecodeForm[*models.SubscriptionRequest](req)
	if err != nil {
		logging.FromContext(req.Context()).Error("Could not decode submitted subscription request request.",
			slog.Any("error", err))
		return
	}

	if !valid {
		ctx := models.SubscriptionRequestToCtx(req.Context(), subscription)
		if err = htmx.NewResponse().RenderTempl(ctx, res, content.AddSubscriptionForm()); err != nil {
			logging.FromContext(req.Context()).Error("Cannot display content.",
				slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
			http.Error(res, "Problem!", http.StatusInternalServerError)
		}

		return
	}

	spew.Dump(*subscription)

	// warnings, err := s.API.elastic.UserActionAddSubscriptions(req.Context(), *subscription)
	// if err != nil {
	// 	logging.FromContext(req.Context()).Error("Could not add subscription.",
	// 		slog.Any("error", err))
	// }

	// if len(warnings) > 0 {
	// 	if err := htmx.NewResponse().RenderTempl(req.Context(), res, content.AddSubscriptionWarning(warnings)); err != nil {
	// 		logging.FromContext(req.Context()).Error("Cannot display content.",
	// 			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
	// 		http.Error(res, "Problem!", http.StatusInternalServerError)
	// 	}

	// 	return
	// }

	if err := htmx.NewResponse().RenderTempl(req.Context(), res, content.AddSubscriptionSuccess()); err != nil {
		logging.FromContext(req.Context()).Error("Cannot display content.",
			slog.Any("error", errors.Join(ErrRenderTemplateFail, err)))
		http.Error(res, "Problem!", http.StatusInternalServerError)
	}
}

// Edit a new subscription
// (GET /home/subscription/edit/{subID})
func (s Server) GetSubscriptionEdit(w http.ResponseWriter, r *http.Request, subID string) {
	w.WriteHeader(http.StatusNotImplemented)
}

// Process a subscription edit
// (POST /home/subscription/edit/{subID})
func (s Server) PostSubscriptionEdit(w http.ResponseWriter, r *http.Request, subID string) {
	w.WriteHeader(http.StatusNotImplemented)
}
