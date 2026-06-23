// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/paddle"
)

// HandlePaddleWebhook handles incoming webhooks from paddle.
func HandlePaddleWebhook(res http.ResponseWriter, req *http.Request) {
	verifier, err := paddle.NewWebhookClient()
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Could not create paddle webhook client.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Verify the request with the verifier
	ok, err := verifier.Verify(req)
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Error occurred when verifying incoming webhook.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	if !ok {
		slogctx.FromCtx(req.Context()).Error("Invalid webhook.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20))
	if err != nil {
		slogctx.FromCtx(req.Context()).Error("Could read incoming webhook body.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	var webhook paddle.Webhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		slogctx.FromCtx(req.Context()).Error("Unable unmarshal webhook data.",
			slog.Any("error", err),
		)
		res.WriteHeader(http.StatusBadRequest)
		return
	}
	webhook.RawBody = body

	paddle.HandleWebhook(req.Context(), webhook)

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(`{"success": true}`))
}

func HandleChoosePaddleSubscription() http.HandlerFunc {
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

		// If ?_ptxn=txn_01... is present, this will be passed so the Paddle overlay is opened automatically for that
		// transaction.
		transactionID := req.FormValue("_ptxn")
		var planID string
		if frequency := req.FormValue("frequency"); frequency != "" {
			var err error
			planID, err = paddle.GetPriceID(frequency)
			if err != nil {
				HandleInternalError(req.Referer(), &models.APIError{
					InternalError: fmt.Errorf("get price id: %w", err),
					StatusCode:    http.StatusBadRequest,
					UserMessage: models.NewErrorMessage(
						"Unable to render form",
						"There was a problem with the request. Please try again.",
					),
				}).ServeHTTP(res, req)
				return
			}
		}

		checkout := &models.CheckoutRequest{UserSubscriptionType: models.UserSubscriptionTypePaddle}
		if err := checkout.SubscriptionData.FromPaddleCheckout(models.PaddleCheckout{
			PlanID:        planID,
			TransactionID: &transactionID,
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

func handlePaddlePurchase() http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		user := models.UserFromCtx(req.Context())
		if user == nil {
			res.WriteHeader(http.StatusInternalServerError)
			RenderPartial(
				&Notification{
					msg: models.NewErrorMessage(
						"Invalid data",
						"There was a problem processing the checkout. This might be temporary, please try again.",
					),
				},
			).ServeHTTP(res, req)
			return
		}

		frequency := req.FormValue("frequency")
		priceID, err := paddle.GetPriceID(frequency)
		if err != nil {
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

		successURL := config.GetBaseURL() + "/checkout/success"
		res.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(res).Encode(map[string]string{
			"priceId":    priceID,
			"successUrl": successURL,
			"email":      user.GetEmail(),
		})
	}
}
