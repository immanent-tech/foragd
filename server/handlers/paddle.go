// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/paddle"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/slots"
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

type ChooseSubscription struct {
	user  *models.User
	props *templates.CheckoutProps
}

func (t *ChooseSubscription) FullResponse(res http.ResponseWriter, req *http.Request) {
	ctx := slots.WithSlot(req.Context(), slots.Header, templates.PaddleHead())
	templ.Handler(
		templates.CreatePage(
			templates.ChooseSubscriptionPlan(t.user, t.props),
			templates.WithPageTitle("Choose Subscription Plan"),
		)).ServeHTTP(res, req.WithContext(ctx))
}

func HandleChooseSubscription() http.HandlerFunc {
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

		user := models.UserFromCtx(req.Context())
		if user == nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get user: %w", models.ErrCtxValueNotFound),
				StatusCode:    http.StatusInternalServerError,
				UserMessage: models.NewErrorMessage(
					"Unable to render form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		}
		// If ?_ptxn=txn_01... is present, pass it to the template so the
		// Paddle overlay is opened automatically for that transaction.
		if transactionID := req.FormValue("_ptxn"); transactionID != "" {
			RenderExternalPage(&ChooseSubscription{
				user: user,
				props: &templates.CheckoutProps{
					TransactionID: transactionID,
				},
			}).ServeHTTP(res, req)
			return
		}

		plan := req.FormValue("plan")
		if _, err := paddle.GetPriceID(plan); err != nil {
			HandleExternalError(&models.APIError{
				InternalError: fmt.Errorf("get price id: %w", err),
				StatusCode:    http.StatusBadRequest,
				UserMessage: models.NewErrorMessage(
					"Unable to render form",
					"There was a problem with the request. Please try again.",
				),
			}).ServeHTTP(res, req)
			return
		} else {
			// Assign annual pricing as default.
			plan, _ = paddle.GetPriceID("annual")
		}
		RenderExternalPage(&ChooseSubscription{
			user: user,
			props: &templates.CheckoutProps{
				PlanID: plan,
			},
		}).ServeHTTP(res, req)
	}
}

// HandlePurchaseSubscription returns a JSON payload consumed by Paddle.js on the client.
// The actual checkout is opened via Paddle.Checkout.open() in the browser;
// this endpoint validates the price ID and returns it for client-side use.
func HandlePurchaseSubscription() http.HandlerFunc {
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

		plan := req.FormValue("plan") // "monthly" or "annual"
		priceID, err := paddle.GetPriceID(plan)
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
