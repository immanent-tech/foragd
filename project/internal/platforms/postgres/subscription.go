// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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

func (c *Client) GetSubscriptionByURL(ctx context.Context, url string) (models.Subscription, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return models.Subscription{}, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	subscription := models.Subscription{
		UserID: userID,
	}

	if err := c.db.First(&subscription, "url = ?", url).Error; err != nil {
		return subscription, fmt.Errorf("unable to get subscription: %w", err)
	}

	return subscription, nil
}

func (c *Client) AddSubscription(_ context.Context, sub *models.Subscription) error {
	result := c.db.Create(&sub)
	if result.Error != nil {
		return errors.Join(ErrAddSubscription, result.Error)
	}

	c.logger.Debug("Added subscription.",
		slog.String("id", sub.ID),
	)

	return nil
}
