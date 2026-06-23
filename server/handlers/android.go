// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/google/android"
)

func HandleChooseAndroidSubscription() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to render form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// The subscription sku
		var plan string
		if plan = req.FormValue("sku"); plan == "" {
			plan = "foragd_annual"
		}

		checkout := &models.CheckoutRequest{UserSubscriptionType: models.UserSubscriptionTypeAndroid}
		if err := checkout.SubscriptionData.FromAndroidCheckout(models.AndroidCheckout{
			SKU: plan,
		}); err != nil {
			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: fmt.Errorf("generate checkout request: %w", err),
				StatusCode:    http.StatusBadRequest,
				UserMessage: models.NewErrorMessage(
					"Unable to render form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		RenderInternalPage(&ChooseSubscription{
			user:    user,
			request: checkout,
		}).ServeHTTP(res, req)
	}
}

// HandleAndroidPurchase  receives a purchaseToken from the client and verifies it server-side.
func HandleAndroidPurchase() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusForbidden,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
		}

		sku := req.FormValue("sku")
		token := req.FormValue("purchaseToken")

		if sku == "" || token == "" {
			HandleExternalError(&models.APIError{
				InternalError: errors.New("parse form values: sku and/or token missing"),
				StatusCode:    http.StatusBadRequest,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		slogctx.Info(req.Context(), "Verifying purchase.",
			slog.String("user_id", user.GetID()),
			slog.String("sku", sku),
		)

		_, err := android.VerifyAndAcknowledgeSubscription(req.Context(), user, sku, token)
		if err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("billing verification: %w", err),
				StatusCode:    http.StatusBadRequest,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		http.Redirect(res, req, "/checkout/success", http.StatusSeeOther)
	}
}
