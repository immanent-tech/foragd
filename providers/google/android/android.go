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
	"sync"
	"time"

	"cloud.google.com/go/auth/credentials/idtoken"
	"github.com/goforj/godump"
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

var ErrNotFound = errors.New("not found")

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

func GetPriceID(frequency string) (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	if priceID, ok := cfg.Pricing[frequency]; ok {
		return priceID, nil
	}

	return "", fmt.Errorf("%w: price frequency %s", ErrNotFound, frequency)
}

// HandleRTDN handles Real-Time Developer Notifications from Google Pub/Sub.
// Configure a Pub/Sub push subscription pointing at /billing/rtdn in Play Console.
func HandleRTDN(res http.ResponseWriter, req *http.Request) {
	if err := loadConfig(); err != nil {
		slogctx.Error(req.Context(), "Could not listen for notifications.",
			slog.Any("error", err),
		)
		http.Error(res, "Unable to listen for notifications", http.StatusInternalServerError)
		return
	}

	// Verify authentication in production.
	if config.IsProduction() {
		// Get the Cloud Pub/Sub-generated JWT in the "Authorization" header.
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" || len(strings.Split(authHeader, " ")) != 2 {
			http.Error(res, "Missing Authorization header", http.StatusBadRequest)
			return
		}
		token := strings.Split(authHeader, " ")[1]

		// Verify and decode the JWT.
		payload, err := idtoken.Validate(req.Context(), token, config.GetBaseURL()+"/webhooks/googleplay")
		if err != nil {
			http.Error(res, fmt.Sprintf("Invalid Token: %v", err), http.StatusBadRequest)
			return
		}
		if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
			http.Error(res, "Wrong Issuer", http.StatusBadRequest)
			return
		}

		// Ensure that `payload.Claims["email"]` is equal to the expected service account set up in the push
		// subscription settings.
		//
		// Ensure that `payload.Claims["email_verified"]` is set to true.
		if payload.Claims["email"] != cfg.PubSubEmail || payload.Claims["email_verified"] != true {
			http.Error(res, "Unexpected email identity", http.StatusBadRequest)
			return
		}
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

	var notification struct {
		PackageName              string `json:"packageName"`
		EventTimeMillis          string `json:"eventTimeMillis"`
		SubscriptionNotification *struct {
			NotificationType int    `json:"notificationType"`
			PurchaseToken    string `json:"purchaseToken"`
			SubscriptionID   string `json:"subscriptionId"`
		} `json:"subscriptionNotification"`
		OneTimeProductNotification *struct {
			NotificationType int    `json:"notificationType"`
			PurchaseToken    string `json:"purchaseToken"`
			SKU              string `json:"sku"`
		} `json:"oneTimeProductNotification"`
	}

	if err := json.Unmarshal(msg.Message.Data, &notification); err != nil {
		http.Error(res, "bad payload", http.StatusBadRequest)
		return
	}

	slogctx.Debug(req.Context(), "billing: RTDN received", slog.String("package", notification.PackageName))

	if sn := notification.SubscriptionNotification; sn != nil {
		switch sn.NotificationType {
		case 1: // SUBSCRIPTION_RECOVERED
			godump.Dump(notification)
		case 2: // SUBSCRIPTION_RENEWED
			godump.Dump(notification)
		case 3: // SUBSCRIPTION_CANCELED
			godump.Dump(notification)
			// s.revokeByToken(sn.PurchaseToken)
		case 4: // SUBSCRIPTION_PURCHASED
			godump.Dump(notification)
		case 12: // SUBSCRIPTION_EXPIRED
			godump.Dump(notification)
			// s.revokeByToken(sn.PurchaseToken)
		case 13: // SUBSCRIPTION_REVOKED
			godump.Dump(notification)
			// s.revokeByToken(sn.PurchaseToken)
		}
	}
	// Pub/Sub requires a 200 to acknowledge receipt
	res.WriteHeader(http.StatusOK)
}

// VerifyAndAcknowledgeSubscription verifies a subscription purchase.
func VerifyAndAcknowledgeSubscription(
	ctx context.Context,
	user *models.User, sku, token string,
) (*Entitlement, error) {
	if err := initClient(ctx); err != nil {
		return nil, fmt.Errorf("init client: %w", err)
	}

	slogctx.Debug(ctx, "Verifying subscription purchase.",
		slog.String("sku", sku))

	sub, err := client.Purchases.Subscriptions.Get(cfg.PackageName, sku, token).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("verify subscription %s: %w", sku, err)
	}

	// Check expiry
	expiryMs := sub.ExpiryTimeMillis
	expiry := time.UnixMilli(expiryMs)
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("subscription expired at %s", expiry)
	}

	// Acknowledge if needed
	if sub.AcknowledgementState == 0 {
		err = client.Purchases.Subscriptions.Acknowledge(
			cfg.PackageName, sku, token,
			&androidpublisher.SubscriptionPurchasesAcknowledgeRequest{},
		).Context(ctx).Do()
		if err != nil {
			slogctx.FromCtx(ctx).Error("acknowledge subscription failed",
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

	if !user.HasValidSubscription() {
		user.Subscription = &models.User_Subscription{}
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
