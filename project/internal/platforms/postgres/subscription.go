// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

func (c *Client) GetSubscriptions(ctx context.Context) ([]models.Subscription, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	var subscriptions []models.Subscription

	if err := c.db.Where("name <> ?", userID).Find(&subscriptions).Error; err != nil {
		return nil, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (c *Client) AddSubscription(ctx context.Context, item *models.SubscriptionRequest) error {
	userID, err := session.UserID(ctx)
	if err != nil {
		return fmt.Errorf("unable to add subscription: %w", err)
	}

	var feed *models.Feed

	if err = c.db.Transaction(func(tx *gorm.DB) error {
		// Get any existing feed, or, create a new feed.
		if feed, err = c.getFeedByURL(item.Link); err != nil {
			feed, err = models.NewFeedFromURL(item.Link)
			if err != nil {
				return err
			}
		}

		// Create a new subscription.
		subscription, err := models.NewSubscription(item.Name, feed.ID, userID)
		if err != nil {
			return err
		}
		// Add the user to the feed subscribers.
		feed.Subscriptions = append(feed.Subscriptions, *subscription)

		// Commit feed.
		if err := tx.Create(feed).Error; err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	}); err != nil {
		return fmt.Errorf("failed to add subscription: %w", err)
	}

	return nil
}
