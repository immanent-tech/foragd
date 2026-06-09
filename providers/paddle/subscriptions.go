// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package paddle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/PaddleHQ/paddle-go-sdk/v5"
	"github.com/PaddleHQ/paddle-go-sdk/v5/pkg/paddlenotification"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/service"
)

func IsActive(s *models.PaddleSubscription) bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusActive)
}

func IsTrial(s *models.PaddleSubscription) bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionItemStatusTrialing)
}

func IsPastDue(s *models.PaddleSubscription) bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusPastDue)
}

func IsPaused(s *models.PaddleSubscription) bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusPaused)
}

func IsCancelled(s *models.PaddleSubscription) bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusCanceled)
}

// CancelSubscription will cancel a user's subscription.
func CancelSubscription(ctx context.Context, user *models.User) error {
	if err := loadClient(); err != nil {
		return fmt.Errorf("load client: %w", err)
	}

	userSubscription, err := user.Subscription.AsPaddleSubscription()
	if err != nil {
		return fmt.Errorf("get user paddle subscription: %w", err)
	}

	if _, err := client.CancelSubscription(ctx, &paddle.CancelSubscriptionRequest{
		SubscriptionID: userSubscription.SubscriptionID,
	}); err != nil {
		return fmt.Errorf("cancel user subscription: %w", err)
	}

	return nil
}

// UpdateUserSubscription handles updating a user's subscription data.
func UpdateUserSubscription[T subscriptionData](ctx context.Context, user *models.User, subscription T) error {
	subscriptionData, err := user.Subscription.AsPaddleSubscription()
	if err != nil {
		return fmt.Errorf("get user paddle subscription: %w", err)
	}

	if id, err := getSubscriptionID(subscription); err != nil {
		return fmt.Errorf("update id: %w", err)
	} else {
		subscriptionData.SubscriptionID = id
	}
	if name, err := getSubscriptionName(subscription); err != nil {
		return fmt.Errorf("update status: %w", err)
	} else {
		subscriptionData.SubscriptionName = name
	}
	if status, err := getSubscriptionStatus(subscription); err != nil {
		return fmt.Errorf("update status: %w", err)
	} else {
		subscriptionData.SubscriptionStatus = status
	}
	switch start, end, err := getCurrentBillingPeriod(subscription); {
	case err != nil && !errors.Is(err, ErrNotFound):
		return fmt.Errorf("update billing period: %w", err)
	case err != nil && errors.Is(err, ErrNotFound):
		slogctx.FromCtx(ctx).Warn("No billing period for subscription.")
	default:
		subscriptionData.CurrentPeriodStart = &start
		subscriptionData.CurrentPeriodEnd = &end
	}
	switch cancelledAt, err := getCancelledAt(subscription); {
	case err != nil && !errors.Is(err, ErrNotFound):
		return fmt.Errorf("update cancelled at: %w", err)
	case err != nil && errors.Is(err, ErrNotFound):
	default:
		subscriptionData.CancelledAt = &cancelledAt
	}
	if priceID, err := getPriceID(subscription); err != nil {
		return fmt.Errorf("update price id: %w", err)
	} else {
		subscriptionData.PriceID = priceID
	}
	if priceName, err := getPriceName(subscription); err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("update price name: %w", err)
	} else if priceName != "" {
		subscriptionData.PriceName = priceName
	}
	if updatedAt, err := getUpdatedAt(subscription); err != nil {
		return fmt.Errorf("update price id: %w", err)
	} else {
		subscriptionData.UpdatedAt = updatedAt
	}

	if err := user.Subscription.FromPaddleSubscription(subscriptionData); err != nil {
		return fmt.Errorf("update user paddle subscription: %w", err)
	}

	if err := service.UpdateUser(ctx, user, map[string]any{
		"subscription_type": models.UserSubscriptionTypePaddle,
		"subscription":      user.Subscription},
	); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

type subscriptionData interface {
	paddlenotification.SubscriptionCreatedNotification | paddlenotification.SubscriptionNotification
}

func getSubscriptionID[T subscriptionData](subscription T) (string, error) {
	switch v := any(subscription).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		return v.ID, nil
	case paddlenotification.SubscriptionNotification:
		return v.ID, nil
	}
	return "", fmt.Errorf("get id for %T: %w", subscription, ErrNotFound)
}

func getSubscriptionName[T subscriptionData](subscription T) (string, error) {
	switch data := any(subscription).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		if len(data.Items) == 0 {
			return "", fmt.Errorf("get items: %w", ErrNotFound)
		}
		return data.Items[0].Product.Name, nil
	case paddlenotification.SubscriptionNotification:
		if len(data.Items) == 0 {
			return "", fmt.Errorf("get items: %w", ErrNotFound)
		}
		return data.Items[0].Product.Name, nil
	}
	return "", fmt.Errorf("get items for %T: %w", subscription, ErrNotFound)
}

func getSubscriptionStatus[T subscriptionData](subscription T) (string, error) {
	switch data := any(subscription).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		return string(data.Status), nil
	case paddlenotification.SubscriptionNotification:
		return string(data.Status), nil
	}
	return "", fmt.Errorf("get status for %T: %w", subscription, ErrNotFound)
}

func getCurrentBillingPeriod[T subscriptionData](s T) (time.Time, time.Time, error) {
	var (
		startStr string
		endStr   string
	)
	switch data := any(s).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		if data.CurrentBillingPeriod == nil {
			return time.Time{}, time.Time{}, fmt.Errorf("get billing period: %w", ErrNotFound)
		}
		startStr = data.CurrentBillingPeriod.StartsAt
		endStr = data.CurrentBillingPeriod.EndsAt
	case paddlenotification.SubscriptionNotification:
		if data.CurrentBillingPeriod == nil {
			return time.Time{}, time.Time{}, fmt.Errorf("get billing period: %w", ErrNotFound)
		}
		startStr = data.CurrentBillingPeriod.StartsAt
		endStr = data.CurrentBillingPeriod.EndsAt
	}
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse billing start: %w", err)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("parse billing end: %w", err)
	}

	return start, end, nil
}

func getPriceID[T subscriptionData](subscription T) (string, error) {
	switch data := any(subscription).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		if len(data.Items) == 0 {
			return "", fmt.Errorf("get items: %w", ErrNotFound)
		}
		return data.Items[0].Price.ID, nil
	case paddlenotification.SubscriptionNotification:
		if len(data.Items) == 0 {
			return "", fmt.Errorf("get items: %w", ErrNotFound)
		}
		return data.Items[0].Price.ID, nil
	}
	return "", fmt.Errorf("get items for %T: %w", subscription, ErrNotFound)
}

func getPriceName[T subscriptionData](subscription T) (string, error) {
	switch data := any(subscription).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		if len(data.Items) == 0 {
			return "", fmt.Errorf("get items: %w", ErrNotFound)
		}
		if data.Items[0].Price.Name != nil {
			return *data.Items[0].Price.Name, nil
		}
	case paddlenotification.SubscriptionNotification:
		if len(data.Items) == 0 {
			return "", fmt.Errorf("get items: %w", ErrNotFound)
		}
		if data.Items[0].Price.Name != nil {
			return *data.Items[0].Price.Name, nil
		}
	}
	return "", fmt.Errorf("get items for %T: %w", subscription, ErrNotFound)
}

func getCancelledAt[T subscriptionData](s T) (time.Time, error) {
	var cancelStr string
	switch data := any(s).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		if data.CanceledAt != nil {
			cancelStr = *data.CanceledAt
		}
	case paddlenotification.SubscriptionNotification:
		if data.CanceledAt != nil {
			cancelStr = *data.CanceledAt
		}
	}

	if cancelStr == "" {
		return time.Time{}, fmt.Errorf("get canceled at: %w", ErrNotFound)
	}

	cancelledAt, err := time.Parse(time.RFC3339, cancelStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cancelled at: %w", err)
	}

	return cancelledAt, nil
}

func getUpdatedAt[T subscriptionData](subscription T) (time.Time, error) {
	var updatedStr string
	switch data := any(subscription).(type) {
	case paddlenotification.SubscriptionCreatedNotification:
		updatedStr = data.UpdatedAt
	case paddlenotification.SubscriptionNotification:
		updatedStr = data.UpdatedAt
	}

	updatedAt, err := time.Parse(time.RFC3339, updatedStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse cancelled at: %w", err)
	}

	return updatedAt, nil
}
