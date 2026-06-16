// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/slots"
)

type ChooseSubscription struct {
	user    *models.User
	request *models.CheckoutRequest
}

func (t *ChooseSubscription) FullResponse(res http.ResponseWriter, req *http.Request) {
	ctx := slots.WithSlot(req.Context(), slots.Header, templates.PaddleHead())
	templ.Handler(
		templates.CreatePage(
			templates.ChooseSubscriptionPlan(t.user, t.request),
			templates.WithPageTitle("Choose Subscription Plan"),
		)).ServeHTTP(res, req.WithContext(ctx))
}

func HandleChooseSubscription() http.HandlerFunc {
	paddleChoice := handleChoosePaddleSubscription()

	return func(res http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("parse form: %w", err),
				StatusCode:    http.StatusBadRequest,
				UserMessage: models.NewErrorMessage(
					"Unable to render form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}

		switch models.ClientTypeFromCtx(req.Context()) {
		case models.ClientTypeTwa:
		default:
			paddleChoice.ServeHTTP(res, req)
		}
	}
}

// HandlePurchaseSubscription returns a JSON payload consumed by Paddle.js on the client.
// The actual checkout is opened via Paddle.Checkout.open() in the browser;
// this endpoint validates the price ID and returns it for client-side use.
func HandlePurchaseSubscription() http.HandlerFunc {
	paddlePurchase := handlePaddlePurchase()

	return func(res http.ResponseWriter, req *http.Request) {
		if err := req.ParseForm(); err != nil {
			res.WriteHeader(http.StatusBadRequest)
			RenderPartial(
				&Notification{
					msg: models.NewWarningMessage(
						"Invalid data",
						"There was a problem processing the checkout. This might be temporary, please try again.",
					),
				},
			).ServeHTTP(res, req)
			return
		}
		switch req.FormValue("subscription_type") {
		case "paddle":
			paddlePurchase.ServeHTTP(res, req)
		case "android":
		default:
			res.WriteHeader(http.StatusBadRequest)
			RenderPartial(
				&Notification{
					msg: models.NewWarningMessage(
						"Invalid data",
						"There was a problem processing the checkout. This might be temporary, please try again.",
					),
				},
			).ServeHTTP(res, req)
			return
		}
	}
}

type PurchaseSubscriptionSuccess struct {
	transactionID string
}

// FullResponse renders the page for the user to choose a subscription plan.
func (t *PurchaseSubscriptionSuccess) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			templates.PurchaseSubscriptionSuccess(t.transactionID),
			templates.WithPageTitle("Choose Subscription Plan"),
		)).ServeHTTP(res, req)
}

func HandlePurchaseSubscriptionSuccess() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		txID := req.URL.Query().Get("_ptxn")
		RenderExternalPage(&PurchaseSubscriptionSuccess{transactionID: txID}).ServeHTTP(res, req)
	}
}
