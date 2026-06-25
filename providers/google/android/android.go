// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package android

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Android Billing.
	ConfigEnvPrefix = "ANDROID_"
)

var ErrNotFound = errors.New("not found")
var ErrClientNotStarted = errors.New("client not started")

// Config is the configuration for Android Billing.
type Config struct {
	// PackageName is what the app package is named in the Google Play Store.
	PackageName string            `koanf:"packagename" validate:"required"`
	PubSubEmail string            `koanf:"pubsubemail" validate:"required,email"`
	Pricing     map[string]string `koanf:"pricing"     validate:"required"`
}

var cfg = Config{
	Pricing: make(map[string]string),
}

var loadConfig = sync.OnceValue(func() error {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load from envrionment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

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

var client *androidpublisher.Service

func StartClient(ctx context.Context) error {
	if err := loadConfig(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	var err error
	client, err = androidpublisher.NewService(ctx,
		option.WithScopes(androidpublisher.AndroidpublisherScope),
	)
	if err != nil {
		return fmt.Errorf("load android billing: %w", err)
	}
	slogctx.FromCtx(ctx).Info("Android billing client created.")
	return nil
}

func GetPriceID(frequency string) (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	if priceID, ok := cfg.Pricing[frequency]; ok {
		return priceID, nil
	}

	return "", fmt.Errorf("%w: price frequency %s", ErrNotFound, frequency)
}

func IsValidSKU(sku string) bool {
	if err := loadConfig(); err != nil {
		return false
	}

	validSKU := false
	for _, configuredID := range cfg.Pricing {
		if configuredID == sku {
			validSKU = true
			break
		}
	}
	return validSKU
}

// VerifyAndAcknowledgeSubscription verifies a subscription purchase.
func VerifyAndAcknowledgeSubscription(
	ctx context.Context,
	user *models.User, sku, token string,
) (*Entitlement, error) {
	if client == nil {
		return nil, ErrClientNotStarted
	}

	slogctx.Debug(ctx, "Verifying subscription purchase.",
		slog.String("sku", sku))

	purchase, err := client.Purchases.Subscriptionsv2.
		Get(cfg.PackageName, token).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("verify subscription %s: %w", sku, err)
	}

	// Check there is an actual item purchased.
	if len(purchase.LineItems) == 0 {
		return nil, fmt.Errorf("subscription %s has no line items", sku)
	}

	// Check state is valid.
	if !isGrantableState(purchase.SubscriptionState) {
		return nil, fmt.Errorf("subscription %s not in a grantable state: %s", sku, purchase.SubscriptionState)
	}

	// Check expiry is valid.
	expiry, err := parseStrTime(purchase.LineItems[0].ExpiryTime)
	if err != nil {
		return nil, fmt.Errorf("parse expiry for %s: %w", sku, err)
	}
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("subscription %s expired at %s", sku, expiry)
	}

	slogctx.Debug(ctx, "Acknowledging subscription purchase.",
		slog.String("sku", sku))

	// Acknowledge if needed
	if purchase.AcknowledgementState != "ACKNOWLEDGEMENT_STATE_ACKNOWLEDGED" {
		if err := acknowledgeSubscriptionPurchase(ctx, cfg.PackageName, sku, token); err != nil {
			slogctx.Warn(ctx, "Acknowledge subscription failed",
				slog.String("sku", sku),
				slog.Any("error", err),
			)
		}
	}

	// Create subscription and associated with user.
	ent, err := createSubscription(ctx, user, sku, token, time.Now().UTC(), expiry)
	if err != nil {
		return nil, fmt.Errorf("create subscription %w", err)
	}
	slogctx.Info(ctx, "Subscription created,",
		slog.String("user_id", user.GetID()),
		slog.String("sku", ent.SKU),
		slog.Time("granted_at", ent.GrantedAt),
		slog.Time("expires_at", *ent.ExpiresAt),
	)
	return ent, nil
}

func acknowledgeSubscriptionPurchase(ctx context.Context, pkg, sku, token string) error {
	if client == nil {
		return ErrClientNotStarted
	}

	if err := client.Purchases.Subscriptions.Acknowledge(
		pkg, sku, token,
		&androidpublisher.SubscriptionPurchasesAcknowledgeRequest{},
	).Context(ctx).Do(); err != nil {
		return fmt.Errorf("acknowledge subscription purchase: %w", err)
	}

	return nil
}

func parseStrTime(strTime string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strTime)
}

// isGrantableState returns true if the subscription state permits granting access.
// SUBSCRIPTION_STATE_CANCELED is included because a cancelled subscription still
// retains access until its expiry time.
func isGrantableState(state string) bool {
	switch state {
	case "SUBSCRIPTION_STATE_ACTIVE",
		"SUBSCRIPTION_STATE_IN_GRACE_PERIOD",
		"SUBSCRIPTION_STATE_CANCELED":
		return true
	default:
		// SUBSCRIPTION_STATE_PAUSED, SUBSCRIPTION_STATE_ON_HOLD,
		// SUBSCRIPTION_STATE_EXPIRED, SUBSCRIPTION_STATE_PENDING, etc.
		return false
	}
}
