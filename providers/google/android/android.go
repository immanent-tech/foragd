// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package android

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/api/androidpublisher/v3"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Android Billing.
	ConfigEnvPrefix = "ANDROID_"
)

// Config is the configuration for Android Billing.
type Config struct {
	// PackageName is what the app package is named in the Google Play Store.
	PackageName string `koanf:"package_name" validate:"required"`
}

var cfg Config

var loadConfig = sync.OnceValue(func() error {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load from envrionment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	slog.Info("Android Billing config loaded.") //nolint:sloglint // we don't pass a context.
	return nil
})

// PurchaseState mirrors the Play API integer values.
type PurchaseState int

const (
	PurchaseStatePurchased PurchaseState = 0
	PurchaseStateCancelled PurchaseState = 1
	PurchaseStatePending   PurchaseState = 2
)

// Entitlement represents a verified, active entitlement for a user.
type Entitlement struct {
	UserID        string
	SKU           string
	PurchaseToken string
	ExpiresAt     *time.Time // nil for lifetime/one-time purchases
	GrantedAt     time.Time
}

// SKUDetails holds Play Store product metadata.
type SKUDetails struct {
	ID          string
	Title       string
	Description string
	Price       string
	Currency    string
	Type        string // "subs" or "inapp"
}

var client *androidpublisher.Service

func initClient(ctx context.Context) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	var err error
	client, err = androidpublisher.NewService(ctx)
	if err != nil {
		return fmt.Errorf("load android billing: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Android billing client created.")
	return nil
}

// VerifyAndAcknowledgeProduct verifies a one-time product purchase and acknowledges it.
// Must be called within 3 days of purchase or Google will refund automatically.
func VerifyAndAcknowledgeProduct(
	ctx context.Context,
	user *models.User, sku, token string,
) (*Entitlement, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	purchase, err := client.Purchases.Products.Get(cfg.PackageName, sku, token).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("billing: verify product %s: %w", sku, err)
	}

	if PurchaseState(purchase.PurchaseState) != PurchaseStatePurchased {
		return nil, fmt.Errorf("billing: purchase not in purchased state: %d", purchase.PurchaseState)
	}

	// Acknowledge if not already done (acknowledgement is idempotent)
	if purchase.AcknowledgementState == 0 {
		err = client.Purchases.Products.Acknowledge(
			cfg.PackageName, sku, token,
			&androidpublisher.ProductPurchasesAcknowledgeRequest{},
		).Context(ctx).Do()
		if err != nil {
			// Log but don't fail — we can retry acknowledgement later
			slogctx.FromCtx(ctx).Error("Billing: acknowledge product failed.",
				slog.String("sku", sku),
				slog.Any("error", err),
			)
		}
	}

	ent := &Entitlement{
		UserID:        user.GetID(),
		SKU:           sku,
		PurchaseToken: token,
		GrantedAt:     time.Now().UTC(),
	}

	if err := user.Subscription.FromAndroidSubscription(models.AndroidSubscription{
		PurchaseToken: token,
		SKU:           sku,
		GrantedAt:     time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("update user android subscription: %w", err)
	}
	if err := service.UpdateUser(ctx, user, map[string]any{
		"subscription_type": models.UserSubscriptionTypeAndroid,
		"subscription":      user.Subscription},
	); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return ent, nil
}

// VerifyAndAcknowledgeSubscription verifies a subscription purchase.
func VerifyAndAcknowledgeSubscription(
	ctx context.Context,
	user *models.User, sku, token string,
) (*Entitlement, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	sub, err := client.Purchases.Subscriptions.Get(cfg.PackageName, sku, token).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("billing: verify subscription %s: %w", sku, err)
	}

	// Check expiry
	expiryMs := sub.ExpiryTimeMillis
	expiry := time.UnixMilli(expiryMs)
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("billing: subscription expired at %s", expiry)
	}

	// Acknowledge if needed
	if sub.AcknowledgementState == 0 {
		err = client.Purchases.Subscriptions.Acknowledge(
			cfg.PackageName, sku, token,
			&androidpublisher.SubscriptionPurchasesAcknowledgeRequest{},
		).Context(ctx).Do()
		if err != nil {
			slogctx.FromCtx(ctx).Error("billing: acknowledge subscription failed",
				slog.String("sku", sku),
				slog.Any("error", err),
			)
		}
	}

	ent := &Entitlement{
		UserID:        user.GetID(),
		SKU:           sku,
		PurchaseToken: token,
		ExpiresAt:     &expiry,
		GrantedAt:     time.Now(),
	}

	if err := user.Subscription.FromAndroidSubscription(models.AndroidSubscription{
		PurchaseToken: token,
		SKU:           sku,
		GrantedAt:     time.Now().UTC(),
		ExpiresAt:     &expiry,
	}); err != nil {
		return nil, fmt.Errorf("update user android subscription: %w", err)
	}
	if err := service.UpdateUser(ctx, user, map[string]any{
		"subscription_type": models.UserSubscriptionTypeAndroid,
		"subscription":      user.Subscription},
	); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return ent, nil
}

// // GetEntitlement returns the current entitlement for a user, if any
// func (s *PlayBillingService) GetEntitlement(userID string) *Entitlement {
// 	s.mu.RLock()
// 	defer s.mu.RUnlock()

// 	ent, ok := s.entitlements[userID]
// 	if !ok {
// 		return nil
// 	}

// 	// Check if subscription has expired
// 	if ent.ExpiresAt != nil && time.Now().After(*ent.ExpiresAt) {
// 		return nil
// 	}

// 	return ent
// }

// // HandleRTDN handles Real-Time Developer Notifications from Google Pub/Sub.
// // Configure a Pub/Sub push subscription pointing at /billing/rtdn in Play Console.
// func (s *PlayBillingService) HandleRTDN(w http.ResponseWriter, r *http.Request) {
// 	var msg struct {
// 		Message struct {
// 			Data []byte `json:"data"`
// 		} `json:"message"`
// 	}

// 	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
// 		http.Error(w, "bad request", http.StatusBadRequest)
// 		return
// 	}

// 	var notification struct {
// 		PackageName              string `json:"packageName"`
// 		EventTimeMillis          string `json:"eventTimeMillis"`
// 		SubscriptionNotification *struct {
// 			NotificationType int    `json:"notificationType"`
// 			PurchaseToken    string `json:"purchaseToken"`
// 			SubscriptionID   string `json:"subscriptionId"`
// 		} `json:"subscriptionNotification"`
// 		OneTimeProductNotification *struct {
// 			NotificationType int    `json:"notificationType"`
// 			PurchaseToken    string `json:"purchaseToken"`
// 			SKU              string `json:"sku"`
// 		} `json:"oneTimeProductNotification"`
// 	}

// 	if err := json.Unmarshal(msg.Message.Data, &notification); err != nil {
// 		http.Error(w, "bad payload", http.StatusBadRequest)
// 		return
// 	}

// 	s.logger.Info("billing: RTDN received", "package", notification.PackageName)

// 	if sn := notification.SubscriptionNotification; sn != nil {
// 		switch sn.NotificationType {
// 		case 1: // SUBSCRIPTION_RECOVERED
// 		case 2: // SUBSCRIPTION_RENEWED
// 			s.logger.Info("billing: subscription renewed", "sku", sn.SubscriptionID)
// 		case 3: // SUBSCRIPTION_CANCELED
// 			s.revokeByToken(sn.PurchaseToken)
// 		case 4: // SUBSCRIPTION_PURCHASED
// 		case 12: // SUBSCRIPTION_EXPIRED
// 			s.revokeByToken(sn.PurchaseToken)
// 		case 13: // SUBSCRIPTION_REVOKED
// 			s.revokeByToken(sn.PurchaseToken)
// 		}
// 	}

// 	// Pub/Sub requires a 200 to acknowledge receipt
// 	w.WriteHeader(http.StatusOK)
// }
