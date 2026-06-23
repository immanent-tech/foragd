// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package android

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"cloud.google.com/go/auth/credentials/idtoken"
	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/api/androidpublisher/v3"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
)

const (
	// SubscriptionRecovered indicates a subscription was recovered from account hold or resumed from pause.
	SubscriptionRecovered SubscriptionNotificationType = 1
	// SubscriptionRenewed indicates an active subscription was renewed.
	SubscriptionRenewed SubscriptionNotificationType = 2
	// SubscriptionCanceled indicates a subscription was either voluntarily or involuntarily cancelled. For voluntary
	// cancellation, sent when the user cancels.
	SubscriptionCanceled SubscriptionNotificationType = 3
	// SubscriptionPurchased indicates a new subscription was purchased.
	SubscriptionPurchased SubscriptionNotificationType = 4
	// SubscriptionOnHold indicates a subscription has entered account hold (if enabled).
	SubscriptionOnHold SubscriptionNotificationType = 5
	// SubscriptionInGracePeriod indicates a subscription has entered grace period (if enabled).
	SubscriptionInGracePeriod SubscriptionNotificationType = 6
	// SubscriptionRestarted indicates a user has restored their subscription from Play > Account > Subscriptions. The
	// subscription was canceled but had not expired yet when the user restores. For more information, see Restorations.
	SubscriptionRestarted SubscriptionNotificationType = 7
	// SubscriptionPaused indicates a subscription has been paused.
	SubscriptionPaused SubscriptionNotificationType = 10
	// SubscriptionRevoked indicates a subscription has been revoked from the user before the expiration time.
	SubscriptionRevoked SubscriptionNotificationType = 12
	// SubscriptionExpired indicates a subscription has expired.
	SubscriptionExpired SubscriptionNotificationType = 13
)

type SubscriptionNotificationType int

type rtdnNotification struct {
	PackageName              string                    `json:"packageName"`
	EventTimeMillis          string                    `json:"eventTimeMillis"`
	SubscriptionNotification *subscriptionNotification `json:"subscriptionNotification"`
}

// https://developer.android.com/google/play/billing/rtdn-reference#sub
type subscriptionNotification struct {
	NotificationType SubscriptionNotificationType `json:"notificationType"`
	PurchaseToken    string                       `json:"purchaseToken"`
}

// HandleRTDN handles Real-Time Developer Notifications from Google Pub/Sub.
// Configure a Pub/Sub push subscription pointing at /billing/rtdn in Play Console.
//
// https://developer.android.com/google/play/billing/rtdn-reference
func HandleRTDN(res http.ResponseWriter, req *http.Request) {
	if client == nil {
		slogctx.Error(req.Context(), "Could not listen for notifications.",
			slog.Any("error", ErrClientNotStarted),
		)
		http.Error(res, "Unable to listen for notifications", http.StatusInternalServerError)
		return
	}

	if err := loadConfig(); err != nil {
		slogctx.Error(req.Context(), "Could not listen for notifications.",
			slog.Any("error", err),
		)
		http.Error(res, "Unable to listen for notifications", http.StatusInternalServerError)
		return
	}

	// Validate the notification.
	if err := validateNotification(req); err != nil {
		slogctx.Error(req.Context(), "Notification validation failed.",
			slog.Any("error", err))
		http.Error(res, "invalid notification", http.StatusBadRequest)
		return
	}

	var msg struct {
		Message struct {
			Data []byte `json:"data"`
		} `json:"message"`
	}

	if err := json.NewDecoder(req.Body).Decode(&msg); err != nil {
		http.Error(res, "bad request", http.StatusBadRequest)
		return
	}

	var notification rtdnNotification
	if err := json.Unmarshal(msg.Message.Data, &notification); err != nil {
		http.Error(res, "bad payload", http.StatusBadRequest)
		return
	}

	slogctx.Debug(req.Context(), "RTDN notification received", slog.String("package", notification.PackageName))

	if sn := notification.SubscriptionNotification; sn != nil {
		// Lookup purchase details.
		purchase, err := lookupSubscriptionFromRTDN(req.Context(), sn.PurchaseToken)
		if err != nil {
			slogctx.Error(req.Context(), "RTDN lookup purchase failed", slog.Any("error", err))
			res.WriteHeader(http.StatusOK)
			return
		}
		// Get user by purchase token.
		user, err := getUserByPurchaseToken(req.Context(), sn.PurchaseToken)
		if err != nil {
			// Token not found directly — this may be a new token from a plan
			// change. Try the linked (old) token from the purchase record.
			if purchase.LinkedPurchaseToken != "" {
				user, err = getUserByPurchaseToken(req.Context(), purchase.LinkedPurchaseToken)
			}
			if err != nil {
				slogctx.Error(req.Context(), "RTDN lookup user failed",
					slog.Any("error", err),
					slog.String("purchase_token", sn.PurchaseToken),
					slog.String("linked_token", purchase.LinkedPurchaseToken),
				)
				res.WriteHeader(http.StatusOK)
				return
			}
			slogctx.Info(req.Context(), "RTDN resolved user via linked purchase token",
				slog.String("old_token", purchase.LinkedPurchaseToken),
				slog.String("new_token", sn.PurchaseToken),
			)
		}
		var subscription models.AndroidSubscription
		if user.Subscription != nil {
			// Get user subscription details.
			if subscription, err = user.Subscription.AsAndroidSubscription(); err != nil {
				slogctx.Error(req.Context(), "RTDN get user subscription failed", slog.Any("error", err))
				res.WriteHeader(http.StatusOK) // ack anyway — don't make Pub/Sub retry forever
				return
			}
		} else {
			slogctx.Error(req.Context(), "RTDN no existing user subscription found")
			res.WriteHeader(http.StatusOK) // ack anyway — don't make Pub/Sub retry forever
			return
		}

		// Acknowledge the purchase if needed.
		if purchase.AcknowledgementState != "ACKNOWLEDGEMENT_STATE_ACKNOWLEDGED" {
			slogctx.Debug(req.Context(), "Acknowledging subscription purchase.",
				slog.String("sku", subscription.SKU))
			if err := acknowledgeSubscriptionPurchase(
				req.Context(),
				cfg.PackageName,
				subscription.SKU,
				sn.PurchaseToken,
			); err != nil {
				slogctx.Warn(req.Context(), "RTDN unable to acknowledge subscription", slog.Any("error", err))
			}
		}

		// Handle notification appropriately.
		switch {
		case !isGrantableState(purchase.SubscriptionState):
			slogctx.Debug(req.Context(), "Revoking subscription entitlement.",
				slog.String("sku", subscription.SKU),
				slog.String("state", purchase.SubscriptionState))
			if err := revokeSubscription(req.Context(), user); err != nil {
				slogctx.Error(req.Context(), "RTDN revoke subscription failed", slog.Any("error", err))
			}

		default:
			slogctx.Debug(req.Context(), "Updating subscription entitlement.",
				slog.String("sku", subscription.SKU),
				slog.String("state", purchase.SubscriptionState))
			if err := updateSubscription(req.Context(), user, purchase, sn.PurchaseToken); err != nil {
				slogctx.Error(req.Context(), "RTDN update subscription failed",
					slog.Any("error", err))
			}
		}
	}
	// Pub/Sub requires a 200 to acknowledge receipt
	res.WriteHeader(http.StatusOK)
}

// validateNotification validates the notification received from the rtdn pubsub subscription.
func validateNotification(req *http.Request) error {
	// Verify authentication in production.
	if config.IsProduction() {
		// Get the Cloud Pub/Sub-generated JWT in the "Authorization" header.
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" || len(strings.Split(authHeader, " ")) != 2 {
			return errors.New("missing Authorization header")
		}
		token := strings.Split(authHeader, " ")[1]

		// Verify and decode the JWT.
		payload, err := idtoken.Validate(req.Context(), token, config.GetBaseURL()+"/webhooks/googleplay")
		if err != nil {
			return fmt.Errorf("invalid token: %w", err)
		}
		if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
			return errors.New("wrong issuer")
		}

		// Ensure that `payload.Claims["email"]` is equal to the expected service account set up in the push
		// subscription settings.
		//
		// Ensure that `payload.Claims["email_verified"]` is set to true.
		if payload.Claims["email"] != cfg.PubSubEmail || payload.Claims["email_verified"] != true {
			return errors.New("unexpected email identity")
		}
	}

	return nil
}

// lookupSubscriptionFromRTDN retrieves the purchase details for a subscription from the given token.
func lookupSubscriptionFromRTDN(
	ctx context.Context,
	purchaseToken string,
) (*androidpublisher.SubscriptionPurchaseV2, error) {
	if client == nil {
		return nil, ErrClientNotStarted
	}

	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	purchase, err := client.Purchases.Subscriptionsv2.
		Get(cfg.PackageName, purchaseToken).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("lookup subscription v2: %w", err)
	}

	return purchase, nil
}
