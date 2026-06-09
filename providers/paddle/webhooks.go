// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package paddle

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	paddle "github.com/PaddleHQ/paddle-go-sdk/v5"
	"github.com/PaddleHQ/paddle-go-sdk/v5/pkg/paddlenotification"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/resend"
	"github.com/immanent-tech/foragd/service"
)

// WebhookClient is a client that handles decoding and verifying incoming Paddle webhooks.
type WebhookClient struct {
	*paddle.WebhookVerifier
}

// NewWebhookClient creates a new webhook client.
var NewWebhookClient = sync.OnceValues(func() (*WebhookClient, error) {
	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &WebhookClient{WebhookVerifier: paddle.NewWebhookVerifier(cfg.WebhookSecret)}, nil
})

// Webhook represents an incoming webhook event. It is a stripped down version of the payload used to identify the event
// that can then be parsed appropriately.
type Webhook struct {
	EventID   string                           `json:"event_id"`
	EventType paddlenotification.EventTypeName `json:"event_type"`
	RawBody   []byte                           `json:"-"`
}

func HandleWebhook(ctx context.Context, webhook Webhook) {
	ctx = slogctx.With(ctx, "event_id", webhook.EventID)
	ctx = slogctx.With(ctx, "event_type", webhook.EventType)

	switch webhook.EventType {
	case paddlenotification.EventTypeNameCustomerCreated:
		var customer paddlenotification.CustomerCreated
		if err := json.Unmarshal(webhook.RawBody, &customer); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to unmarshal customer created event.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "customer_id", customer.Data.ID)

		// Retrieve the user associated with the customer email.
		user, err := service.GetUserByEmail(ctx, customer.Data.Email)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable to find existing user for new customer.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "user_id", user.GetID())

		// Update the user's subscription.
		userSubscription, err := user.Subscription.AsPaddleSubscription()
		if err != nil {
			slogctx.FromCtx(ctx).Error("Get user subscription failed.",
				slog.Any("error", err),
			)
			return
		}

		userSubscription.CustomerID = customer.Data.ID
		if err := user.Subscription.FromPaddleSubscription(userSubscription); err != nil {
			slogctx.FromCtx(ctx).Error("Update user subscription failed.",
				slog.Any("error", err),
			)
			return
		}

		if err := service.UpdateUser(ctx, user, map[string]any{
			"subscription_type": models.UserSubscriptionTypePaddle,
			"subscription":      user.Subscription},
		); err != nil {
			slogctx.FromCtx(ctx).Error("Could not update user.",
				slog.Any("error", err),
			)
			return
		}

		slogctx.FromCtx(ctx).Info("Added customer details to user.")

	case paddlenotification.EventTypeNameSubscriptionCreated:
		var subscription paddlenotification.SubscriptionCreated
		if err := json.Unmarshal(webhook.RawBody, &subscription); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to unmarshal subscription created event.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "subscription_id", subscription.Data.ID)
		ctx = slogctx.With(ctx, "customer_id", subscription.Data.CustomerID)

		user, err := GetUserByCustomerID(ctx, subscription.Data.CustomerID)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable get user for customer.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "user_id", user.GetID())

		if err := UpdateUserSubscription(ctx, user, subscription.Data); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to create subscription for user.",
				slog.Any("error", err),
			)
			return
		}

		slogctx.FromCtx(ctx).Debug("Subscription created.",
			slog.String("status", string(subscription.Data.Status)),
		)

	case paddlenotification.EventTypeNameSubscriptionUpdated:
		var subscription paddlenotification.SubscriptionUpdated
		if err := json.Unmarshal(webhook.RawBody, &subscription); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to unmarshal subscription updated event.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "subscription_id", subscription.Data.ID)
		ctx = slogctx.With(ctx, "customer_id", subscription.Data.CustomerID)

		user, err := GetUserByCustomerID(ctx, subscription.Data.CustomerID)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable get user for customer.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "user_id", user.GetID())

		if err := UpdateUserSubscription(ctx, user, subscription.Data); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to update subscription for user.",
				slog.Any("error", err),
			)
			return
		}

	case paddlenotification.EventTypeNameSubscriptionCanceled:
		var subscription paddlenotification.SubscriptionCanceled
		if err := json.Unmarshal(webhook.RawBody, &subscription); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to unmarshal subscription cancelled event.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "subscription_id", subscription.Data.ID)
		ctx = slogctx.With(ctx, "customer_id", subscription.Data.CustomerID)

		user, err := GetUserByCustomerID(ctx, subscription.Data.CustomerID)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable get user for customer.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "user_id", user.GetID())

		if err := UpdateUserSubscription(ctx, user, subscription.Data); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to cancel subscription for user.",
				slog.Any("error", err),
			)
			return
		}

	case paddlenotification.EventTypeNameTransactionCompleted:
		var transaction paddlenotification.TransactionCompleted
		if err := json.Unmarshal(webhook.RawBody, &transaction); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to unmarshal transaction completed event.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "transaction_id", transaction.Data.ID)
		if transaction.Data.CustomerID == nil {
			return
		}

		ctx = slogctx.With(ctx, "customer_id", transaction.Data.CustomerID)

		user, err := GetUserByCustomerID(ctx, *transaction.Data.CustomerID)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable get user for customer.",
				slog.Any("error", err),
			)
			return
		}
		ctx = slogctx.With(ctx, "user_id", user.GetID())

		// Create and send thank you email to user.
		email, err := resend.NewTemplatedEmail(
			"subscription-thank-you",
			resend.WithTo(user.GetEmail()),
			resend.WithTag(resend.TagCategory, resend.TagCategoryPromotional),
			resend.WithVariable("USER_NICKNAME", user.GetNickname()),
		)
		if err != nil {
			slogctx.FromCtx(ctx).Error("Unable to generate thank you email.",
				slog.Any("error", err),
			)
			return
		}
		if err := resend.SendEmail(ctx, resend.WithExistingEmail(email)); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to send thank you email.",
				slog.Any("error", err),
			)
			return
		}

		slogctx.FromCtx(ctx).Info("Transaction completed.")

	case paddlenotification.EventTypeNameTransactionPaymentFailed:
		var transaction paddlenotification.TransactionPaymentFailed
		if err := json.Unmarshal(webhook.RawBody, &transaction); err != nil {
			slogctx.FromCtx(ctx).Error("Unable to unmarshal transaction failed event.",
				slog.Any("error", err),
			)
			return
		}
		slogctx.FromCtx(ctx).Debug("Transaction payment failed.",
			slog.String("transaction_id", transaction.Data.ID),
			slog.String("customer_id", *transaction.Data.CustomerID),
		)
		// TODO: Notify customer, handle dunning.

	default:
		slogctx.FromCtx(ctx).Warn("Unhandled webhook event.",
			slog.String("event_type", string(webhook.EventType)),
		)
	}

}
