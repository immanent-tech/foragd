// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

var ErrAddSubscription = errors.New("error adding subscription")

func (c *Client) GetAllSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	var subscriptions []models.Subscription

	if err := c.db.Where("user_id = ?", userID).Find(&subscriptions).Error; err != nil {
		return nil, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (c *Client) FindSubscriptionByFeedID(ctx context.Context, feedID string) (*models.Subscription, error) {
	var subscription models.Subscription

	userID, err := session.UserID(ctx)
	if err != nil {
		return &subscription, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	subscription.UserID = userID

	if err := c.db.First(&subscription, "feed_id = ?", feedID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return &subscription, fmt.Errorf("unable to get subscription: %w", err)
	}

	return &subscription, nil
}

func (c *Client) AddSubscription(ctx context.Context, userID string, sub *models.Subscription) error {
	// Get the user object.
	user, err := c.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("unable to add subscriptions: %w", err)
	}
	// Add the new subscription to the user's subscriptions.
	user.Subscriptions = append(user.Subscriptions, *sub)

	result := c.db.Save(&user)
	if result.Error != nil {
		return errors.Join(ErrAddSubscription, result.Error)
	}

	c.logger.Debug("Added subscription.",
		slog.String("subscription_id", sub.ID),
	)

	return nil
}
