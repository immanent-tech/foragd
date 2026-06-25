// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/angelofallars/htmx-go"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
	"github.com/immanent-tech/foragd/web/templates/element"
)

type ChooseSubscription struct {
	user    *models.User
	request *models.CheckoutRequest
}

func (t *ChooseSubscription) FullResponse(res http.ResponseWriter, req *http.Request) {
	ctx := slogctx.With(req.Context(), "client", models.ClientTypeFromCtx(req.Context()))
	templ.Handler(
		templates.CreatePage(
			templates.LayoutInternal(
				&templates.InternalLayoutProps{User: t.user},
				templates.Checkout(t.user, t.request),
			),
			templates.WithPageTitle("Choose a subscription plan"),
		)).ServeHTTP(res, req.WithContext(ctx))
}

func (t *ChooseSubscription) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(
		templates.LayoutInternal(
			&templates.InternalLayoutProps{User: t.user},
			templates.Checkout(t.user, t.request),
		),
		templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Choose a subscription plan")).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func HandleChooseSubscription() http.HandlerFunc {
	paddleChoice := HandleChoosePaddleSubscription()
	androidChoice := HandleChooseAndroidSubscription()

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

func HandlePurchaseSubscription() http.HandlerFunc {
	paddlePurchase := handlePaddlePurchase()
	androidPurchase := HandleAndroidPurchase()

	return func(res http.ResponseWriter, req *http.Request) {
		// Pre-parse form data and fail early if there is no form submission.
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
	user          *models.User
	transactionID string
}

func (t *PurchaseSubscriptionSuccess) FullResponse(res http.ResponseWriter, req *http.Request) {
	ctx := slogctx.With(req.Context(), "client", models.ClientTypeFromCtx(req.Context()))
	templ.Handler(
		templates.CreatePage(
			templates.LayoutInternal(
				&templates.InternalLayoutProps{User: t.user},
				templates.PurchaseSubscriptionSuccess(t.transactionID),
			),
			templates.WithPageTitle("Purchase success!"),
		)).ServeHTTP(res, req.WithContext(ctx))
}

func (t *PurchaseSubscriptionSuccess) PartialResponse(res http.ResponseWriter, req *http.Request) {
	res.Header().Set(htmx.HeaderPushURL, req.URL.String())
	templ.Handler(
		templates.LayoutInternal(
			&templates.InternalLayoutProps{User: t.user},
			templates.PurchaseSubscriptionSuccess(t.transactionID),
		),
		templ.WithFragments(templates.ContentFragment)).ServeHTTP(res, req)
	templ.Handler(templates.UpdateTitle("Purchase success!")).ServeHTTP(res, req)
	templ.Handler(templates.SideBar(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
	templ.Handler(templates.Dock(element.WithHXSwapOOB("true"))).ServeHTTP(res, req)
}

func HandlePurchaseSubscriptionSuccess() http.HandlerFunc {
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

		txID := req.URL.Query().Get("_ptxn")
		RenderInternalPage(&PurchaseSubscriptionSuccess{user: user, transactionID: txID}).ServeHTTP(res, req)
	}
}
