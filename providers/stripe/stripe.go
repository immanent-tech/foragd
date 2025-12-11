// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v83"
	portalsession "github.com/stripe/stripe-go/v83/billingportal/session"
	"github.com/stripe/stripe-go/v83/checkout/session"
	"github.com/stripe/stripe-go/v83/price"
	"github.com/stripe/stripe-go/v83/product"
	"github.com/stripe/stripe-go/v83/subscription"
	"github.com/stripe/stripe-go/webhook"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic"
)

const (
	metadataUserID      = "foragd_user_id"
	metadataAuth0UserID = "auth0_user_id"
)

// ErrInvalidSubscription indicates there was invalid data sent or received about a subscription.
var ErrInvalidSubscription = errors.New("invalid subscription")

// Checkout is alocal wrapper around the stripe.CheckoutSession object.
type Checkout struct {
	*stripe.CheckoutSession
}

// PortalSession is a local wrapper around the stripe.BillingPortalSession object.
type PortalSession struct {
	*stripe.BillingPortalSession
}

// NewCheckoutSession creates a new checkout session, that is used to allow a user to purchase a subscription plan.
func NewCheckoutSession(user *models.User, planID string) (*Checkout, error) {
	if err := loadConfigOnce(); err != nil {
		return nil, fmt.Errorf("load stripe config: %w", err)
	}

	params := &stripe.PriceListParams{
		LookupKeys: stripe.StringSlice([]string{
			planID,
		}),
	}
	i := price.List(params)
	var subscriptionPrice *stripe.Price
	for i.Next() {
		p := i.Price()
		subscriptionPrice = p
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(user.GetID()),
		CustomerEmail:     stripe.String(user.GetEmail()),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		// Subscription level price.
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(subscriptionPrice.ID),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			// Add the user's auth0 id so we can match the subscription to the user.
			Metadata: map[string]string{
				metadataUserID:      user.GetID(),
				metadataAuth0UserID: user.GetExternalID(),
			},
			// Define trial period.z
			TrialPeriodDays: stripe.Int64(trialPeriodDays),
			// Define behaviour at end of trial when no payment details have been entered.
			TrialSettings: &stripe.CheckoutSessionSubscriptionDataTrialSettingsParams{
				EndBehavior: &stripe.CheckoutSessionSubscriptionDataTrialSettingsEndBehaviorParams{
					MissingPaymentMethod: stripe.String(trialEndBehavior),
				},
			},
		},
		// URL to redirect user on successful payment.
		SuccessURL: stripe.String(cfg.BaseURL + "/checkout/success?session_id={CHECKOUT_SESSION_ID}"),
		// URL to redirect user on cancel payment.
		CancelURL: stripe.String(cfg.BaseURL),
		// Automatically calculate and collect appropriate tax amounts.
		AutomaticTax: &stripe.CheckoutSessionAutomaticTaxParams{Enabled: stripe.Bool(true)},
		// Don't require payment (allow a trial without entering payment details).
		PaymentMethodCollection: stripe.String(stripe.CheckoutSessionPaymentMethodCollectionIfRequired),
	}

	s, err := session.New(checkoutParams)
	if err != nil {
		return nil, fmt.Errorf("create new checkout session: %w", ErrInvalidSubscription)
	}

	return &Checkout{CheckoutSession: s}, nil
}

// NewPortalSession creates a new portal session, which allows a user to manage their subscription plan.
func NewPortalSession(sessionID string) (*PortalSession, error) {
	portalSession, err := session.Get(sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("get stripe session id: %w", err)
	}

	// Authenticate your user.
	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(portalSession.Customer.ID),
		ReturnURL: stripe.String(cfg.BaseURL + "/home"),
	}
	ps, err := portalsession.New(params)
	if err != nil {
		return nil, fmt.Errorf("create new portal session: %w", err)
	}

	return &PortalSession{BillingPortalSession: ps}, nil
}

// CancelSubscription will cancel a user's active subscription. Subscription will be cancelled at the end of the current
// billing period.
//
// https://docs.stripe.com/billing/subscriptions/cancel?dashboard-or-api=api#cancel-at-the-end-of-the-current-billing-period
func CancelSubscription(user *models.User) error {
	err := loadConfigOnce()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	params := &stripe.SubscriptionParams{CancelAtPeriodEnd: stripe.Bool(true)}
	_, err = subscription.Update(user.Metadata.StripeSubscriptionID, params)
	if err != nil {
		return fmt.Errorf("update subscription cancel period: %w", err)
	}

	return nil
}

// StopPendingCancellation will stop the pending cancellation of a user's subscription.
//
// https://docs.stripe.com/billing/subscriptions/cancel?dashboard-or-api=api#reactivating-canceled-subscriptions
func StopPendingCancellation(user *models.User) error {
	err := loadConfigOnce()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	params := &stripe.SubscriptionParams{CancelAtPeriodEnd: stripe.Bool(false)}
	_, err = subscription.Update(user.Metadata.StripeSubscriptionID, params)
	if err != nil {
		return fmt.Errorf("stop pending subscription cancellation: %w", err)
	}

	return nil
}

// HandleWebhook will handle incoming webhook requests from Stripe, that are sent in response to events related to a
// user's subscription (like plan changes, trial expiry, payment events).
func HandleWebhook(api *elastic.API) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		const maxBodyBytes = int64(65536)
		bodyReader := http.MaxBytesReader(res, req.Body, maxBodyBytes)
		payload, err := io.ReadAll(bodyReader)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("Error reading webhook request body.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		endpointSecret := cfg.WebHookSecret

		// Verify recieved webhook was sent by Stripe.
		signatureHeader := req.Header.Get("Stripe-Signature")
		event, err := webhook.ConstructEvent(payload, signatureHeader, endpointSecret)
		if err != nil {
			slogctx.FromCtx(req.Context()).Error("⚠️  Webhook signature verification failed.",
				slog.Any("error", err),
			)
			res.WriteHeader(http.StatusBadRequest) // Return a 400 error on a bad signature
			return
		}

		// Unmarshal the event data into an appropriate struct depending on its Type
		switch event.Type {
		case "customer.subscription.deleted":
			var subscription stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Error parsing webhook JSON.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusBadRequest)
				return
			}
			err = handleSubscriptionDeleted(req.Context(), api, subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).
					Error("Handle Webhook: error occurred processing subscription deletion.",
						slog.Any("error", err),
					)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}
		case "customer.subscription.updated":
			var subscription stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Error parsing webhook JSON.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusBadRequest)
				return
			}
			err = handleSubscriptionUpdated(req.Context(), api, subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).
					Error("Handle Webhook: error occurred processing subscription deletion.",
						slog.Any("error", err),
					)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}
		case "customer.subscription.created":
			var subscription stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Error parsing webhook JSON.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusBadRequest)
				return
			}
			slogctx.FromCtx(req.Context()).Debug("New user subscription.")
			// Then define and call a func to handle the successful attachment of a PaymentMethod.
			err = handleSubscriptionCreated(req.Context(), api, subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Error parsing webhook JSON.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusInternalServerError)
				return
			}
		case "customer.subscription.trial_will_end":
			var subscription stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Error parsing webhook JSON.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusBadRequest)
				return
			}
			log.Printf("Subscription trial will end for %d.", subscription.ID)
			// Then define and call a func to handle the successful attachment of a PaymentMethod.
			// handleSubscriptionTrialWillEnd(subscription)
		case "entitlements.active_entitlement_summary.updated":
			var subscription stripe.Subscription
			err := json.Unmarshal(event.Data.Raw, &subscription)
			if err != nil {
				slogctx.FromCtx(req.Context()).Error("Error parsing webhook JSON.",
					slog.Any("error", err),
				)
				res.WriteHeader(http.StatusBadRequest)
				return
			}
			log.Printf("Active entitlement summary updated for %d.", subscription.ID)
			// Then define and call a func to handle active entitlement summary updated.
			// handleEntitlementUpdated(subscription)
		default:
			slogctx.FromCtx(req.Context()).Warn("Unhandled webhook event.",
				slog.String("type", event.Type),
			)
		}
		res.WriteHeader(http.StatusOK)
	}
}

// handleSubscriptionDeleted will update the user metadata with the new subscription status (i.e., cancelled) and set
// the cancelAt timestamp.
func handleSubscriptionDeleted(ctx context.Context, api *elastic.API, subscription stripe.Subscription) error {
	user, err := api.GetUser(ctx, subscription.Metadata[metadataUserID])
	if err != nil {
		return fmt.Errorf("subscription deleted: %w", err)
	}

	// Update subscription plan status.
	metadata := user.Metadata
	metadata.PlanStatus = subscription.Status
	metadata.CancelAt = time.Unix(subscription.CancelAt, 0)

	// Update the user object with the new metadata.
	err = api.UpdateUser(ctx, user.GetID(), map[string]any{
		"metadata": metadata,
	})
	if err != nil {
		return fmt.Errorf("subscription deleted: %w", err)
	}
	return nil
}

// handleSubscriptionUpdated will update the user metadata with any changed subscription plan, update the plan status
// and adjust the max history and updates frequency, if required.
func handleSubscriptionUpdated(ctx context.Context, api *elastic.API, subscription stripe.Subscription) error {
	// Retrieve the product details
	prod, err := product.Get(subscription.Items.Data[0].Price.Product.ID, &stripe.ProductParams{})
	if err != nil {
		return fmt.Errorf("get product details: %w", err)
	}

	user, err := api.GetUser(ctx, subscription.Metadata[metadataUserID])
	if err != nil {
		return fmt.Errorf("get user details: %w", err)
	}

	// Set base metadata
	metadata := models.UserMetadata{
		Plan:                 prod.Name,
		PlanStatus:           subscription.Status,
		PlanID:               prod.ID,
		StripeSubscriptionID: subscription.ID,
		CancelAt:             time.Unix(subscription.CancelAt, 0),
	}
	// Set plan-specific metadata
	switch metadata.Plan {
	case "Gatherer":
		metadata.MaxHistory = models.GathererMaxHistory.String()
		metadata.UpdatesFrequency = models.GathererUpdatesFrequency.String()
	case "Collector":
		metadata.MaxHistory = models.CollectorMaxHistory.String()
		metadata.UpdatesFrequency = models.CollectorUpdatesFrequency.String()
	case "Curator":
		metadata.MaxHistory = models.CuratorMaxHistory.String()
		metadata.UpdatesFrequency = models.CuratorUpdatesFrequency.String()
	}

	// Update the user object with the new metadata.
	err = api.UpdateUser(ctx, user.GetID(), map[string]any{
		"metadata": metadata,
	})
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

// handleSubscriptionCreate will add all details about the new subscription to the user metadata and set appropriate
// values for the max history and updates frequency.
func handleSubscriptionCreated(ctx context.Context, api *elastic.API, subscription stripe.Subscription) error {
	// Retrieve the product details
	prod, err := product.Get(subscription.Items.Data[0].Price.Product.ID, &stripe.ProductParams{})
	if err != nil {
		return fmt.Errorf("get product details: %w", err)
	}

	user, err := api.GetUser(ctx, subscription.Metadata[metadataUserID])
	if err != nil {
		return fmt.Errorf("get user details: %w", err)
	}

	// Set base metadata
	metadata := models.UserMetadata{
		Plan:                 prod.Name,
		PlanStatus:           subscription.Status,
		PlanID:               prod.ID,
		TrialEnd:             time.Unix(subscription.TrialEnd, 0),
		StripeCustomerID:     subscription.Customer.ID,
		StripeSubscriptionID: subscription.ID,
	}
	// Set plan-specific metadata
	switch metadata.Plan {
	case "Gatherer":
		metadata.MaxHistory = models.GathererMaxHistory.String()
		metadata.UpdatesFrequency = models.GathererUpdatesFrequency.String()
	case "Collector":
		metadata.MaxHistory = models.CollectorMaxHistory.String()
		metadata.UpdatesFrequency = models.CollectorUpdatesFrequency.String()
	case "Curator":
		metadata.MaxHistory = models.CuratorMaxHistory.String()
		metadata.UpdatesFrequency = models.CuratorUpdatesFrequency.String()
	}

	// Update the user object with the new metadata.
	err = api.UpdateUser(ctx, user.GetID(), map[string]any{
		"metadata": metadata,
	})
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}
