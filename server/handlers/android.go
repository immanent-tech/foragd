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
	"github.com/immanent-tech/foragd/web/templates"
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
			title: templates.PageTitle{
				Summary:     "Choose Subscription Plan",
				Description: "Pick whether to subscribe monthly or yearly",
			},
			user:    user,
			request: checkout,
		}).ServeHTTP(res, req)
	}
}

// HandleAndroidPurchase receives a purchaseToken from the client and verifies it server-side.
func HandleAndroidPurchase() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusForbidden,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// If the user already has a valid subscription and its an android subscription, redirect to restore the
		// purchase (no-op if not needed). If it is not an android subscription, error.
		if user.HasValidSubscription() {
			sub, err := user.Subscription.AsAndroidSubscription()
			if err == nil && sub.PurchaseToken == req.FormValue("purchaseToken") {
				// Restore of an already-active, already-matching subscription — no-op success.
				http.Redirect(res, req, "/checkout/success", http.StatusSeeOther)
				return
			}

			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: errors.New("user has existing subscription"),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Verify we are processing an android subscription.
		if subscriptionType := req.FormValue("subscription_type"); subscriptionType != "android" {
			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: fmt.Errorf("handle purchase: invalid subscription type %s", subscriptionType),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		sku := req.FormValue("sku")
		token := req.FormValue("purchaseToken")

		// Check SKU is valid.
		if !android.IsValidSKU(sku) {
			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: fmt.Errorf("handle purchase: unrecognised sku %s", sku),
				StatusCode:    http.StatusBadRequest,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Check token hasn't already been granted.
		granted, err := android.TokenAlreadyGranted(req.Context(), token)
		if err != nil && !errors.Is(err, android.ErrNotFound) {
			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: fmt.Errorf("handle purchase: check for existing token %w", err),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		if granted {
			HandleInternalError(req.Referer(), &models.APIError{
				InternalError: fmt.Errorf("handle purchase: token already claimed: %s", token),
				StatusCode:    http.StatusUnprocessableEntity,
				UserMessage: models.NewErrorMessage(
					"Unable to complete purchase",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		// Validate purchase parameters.
		if sku == "" || token == "" {
			HandleInternalError(req.Referer(), &models.APIError{
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
			slog.String("sku", sku),
		)

		_, err = android.VerifyAndAcknowledgeSubscription(req.Context(), user, sku, token)
		if err != nil {
			HandleInternalError(req.Referer(), &models.APIError{
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
