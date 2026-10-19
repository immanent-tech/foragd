// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type ChooseSubscription struct {
	*Page
}

func (m *Manager) HandleChooseSubscription() http.HandlerFunc {
	paddleChoice := m.HandleChoosePaddleSubscription(m.AppConfig)
	androidChoice := m.HandleChooseAndroidSubscription(m.AppConfig)

	return func(res http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			m.HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("parse form: %w", err),
				StatusCode:    http.StatusBadRequest,
				UserMessage: models.NewErrorMessage(
					"Unable to render form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Retrieve the "has_dgs" query parameter, that indicates the browser supports the digitalGoodsService API
		// (i.e., is Android device).
		switch req.FormValue("has_dgs") {
		case "true":
			slogctx.Info(req.Context(), "Showing android subscription options.")
			androidChoice.ServeHTTP(res, req)
		default:
			slogctx.Info(req.Context(), "Showing web subscription options.")
			paddleChoice.ServeHTTP(res, req)
		}
	}
}

func (m *Manager) HandlePurchaseSubscription(userSvc UserService) http.HandlerFunc {
	paddlePurchase := m.handlePaddlePurchase(m.AppConfig)
	androidPurchase := m.HandleAndroidPurchase(userSvc)

	return func(res http.ResponseWriter, req *http.Request) {
		// Pre-parse form data and fail early if there is no form submission.
		if err := req.ParseForm(); err != nil {
			res.WriteHeader(http.StatusBadRequest)
			m.HandleNotification(res, req,
				models.NewWarningMessage(
					"Invalid data",
					"There was a problem processing the checkout. This might be temporary, please try again.",
				),
			)
			return
		}
		// Handle different checkout options as appropriate.
		switch req.FormValue("subscription_type") {
		case "paddle":
			slogctx.Info(req.Context(), "Processing web subscription purchases.")
			paddlePurchase.ServeHTTP(res, req)
		case "android":
			slogctx.Info(req.Context(), "Processing android subscription purchases.")
			androidPurchase.ServeHTTP(res, req)
		default:
			res.WriteHeader(http.StatusBadRequest)
			m.HandleNotification(res, req,
				models.NewWarningMessage(
					"Invalid data",
					"There was a problem processing the checkout. This might be temporary, please try again.",
				),
			)
			return
		}
	}
}

type PurchaseSubscriptionSuccess struct {
	*Page
}

func (m *Manager) HandlePurchaseSubscriptionSuccess() http.HandlerFunc {
	page := &PurchaseSubscriptionSuccess{
		Page: m.NewPage("/checkout/success", WithSummary("Purchase subscription success")),
	}
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			m.HandleInternalError(
				http.StatusInternalServerError,
				fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
			).ServeHTTP(res, req)
			return
		}

		transactionID := validation.SanitizeInput(req.URL.Query().Get("_ptxn"))
		page.template = templates.PurchaseSubscriptionSuccess(transactionID)
		page.Render().ServeHTTP(res, req)
	}
}
