// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package android

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/api/androidpublisher/v3"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/models/schema"
	"github.com/immanent-tech/foragd/providers/elastic"
	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/service"
)

func TokenAlreadyGranted(ctx context.Context, token string) (bool, error) {
	existing, err := getUserByPurchaseToken(ctx, token)
	if err != nil {
		return false, fmt.Errorf("check existing token: %w", err)
	}
	if existing != nil {
		return true, nil
	}
	return false, nil
}

// getUserByPurchaseToken retrieves the user associated with the given purchase token.
func getUserByPurchaseToken(ctx context.Context, token string) (*models.User, error) {
	// Retrieve the user associated with the customer ID.
	resp, err := elastic.Search[*models.User](
		ctx,
		schema.UsersIndexRO(),
		query.Term("subscription.purchase_token", token),
		elastic.WithDocSorting(),
		elastic.WithTrackTotalHits(false),
		elastic.WithSize(1),
	)
	if err != nil {
		return nil, fmt.Errorf("find user by purchase token: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("find user by purchase token: %w", ErrNotFound)
	}
	return resp.Results[0], nil
}

// createSubscription creates a new android subscription and associates it with the user.
func createSubscription(
	ctx context.Context,
	user *models.User,
	sku, token string,
	start time.Time,
	expiry time.Time,
) (*Entitlement, error) {
	user.Subscription = &models.User_Subscription{}
	if err := user.Subscription.FromAndroidSubscription(models.AndroidSubscription{
		PurchaseToken: token,
		SKU:           sku,
		GrantedAt:     start,
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

	return &Entitlement{
		UserID:        user.GetID(),
		SKU:           sku,
		PurchaseToken: token,
		ExpiresAt:     &expiry,
		GrantedAt:     start,
	}, nil
}

// updateSubscription updates the user's android subscription as appropriate.
func updateSubscription(
	ctx context.Context,
	user *models.User,
	purchase *androidpublisher.SubscriptionPurchaseV2,
	token string,
) error {
	if client == nil {
		return ErrClientNotStarted
	}

	// Get user's existing subscription.
	subscription, err := user.Subscription.AsAndroidSubscription()
	if err != nil {
		return fmt.Errorf("get user subscription: %w", err)
	}

	// Update purchase token as needed.
	if subscription.PurchaseToken != token {
		subscription.PurchaseToken = token
	}

	// Update expiry as needed.
	if exp := purchase.LineItems[0].ExpiryTime; exp != "" {
		if newExpiry, err := parseStrTime(exp); err != nil {
			slogctx.Error(ctx, "Could not parse expiry.", slog.Any("error", err))
		} else if subscription.ExpiresAt == nil || !subscription.ExpiresAt.Equal(newExpiry) {
			subscription.ExpiresAt = &newExpiry
		}
	}

	// Update the subscription.
	if err := service.UpdateUser(ctx, user, map[string]any{
		"subscription": user.Subscription},
	); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

// revokeSubscription removes the user's android subscription entitlement.
func revokeSubscription(ctx context.Context, user *models.User) error {
	user.Subscription = nil

	if err := service.UpdateUser(ctx, user, map[string]any{
		"subscription_type": "",
		"subscription":      nil,
	}); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}
