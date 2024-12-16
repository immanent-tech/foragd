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

var (
	ErrAddSubscription = errors.New("error adding subscription(s)")
	ErrGetSubscription = errors.New("error retrieving subscription(s)")
)

// IsSubscribed returns a bool indicating if the user is subscribed to the given
// feed.
func (c *Client) IsSubscribed(ctx context.Context, feedID string) (bool, error) {
	var subscription models.Subscription

	userID, err := session.UserID(ctx)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrGetSubscription, err)
	}

	result := c.db.Where("user_id = ? AND feed_id = ?", userID, feedID).First(&subscription)

	switch {
	case result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound):
		return false, nil
	case result.Error != nil:
		return false, fmt.Errorf("%w: %w", ErrGetSubscription, result.Error)
	default:
		return true, nil
	}
}

// FilterSubscriptionsByFeedID retrieves the user subscriptions, filtered to the
// given feed ids.
func (c *Client) FilterSubscriptionsByFeedID(ctx context.Context, feedIDs ...string) ([]models.Subscription, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGetSubscription, err)
	}

	var subscriptions []models.Subscription

	if err := c.db.Where("user_id = ? AND feed_id IN ?", userID, feedIDs).Find(&subscriptions).Error; err != nil {
		return nil, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	return subscriptions, nil
}

// GetAllSubscriptions retrieves all subscriptions for a user.
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
