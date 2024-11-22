// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/internal/models"
	"github.com/joshuar/go-feed-me/internal/server/session"
)

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

func (c *Client) GetSubscription(ctx context.Context, subID string) (models.Subscription, error) {
	userID, err := session.UserID(ctx)
	if err != nil {
		return models.Subscription{}, fmt.Errorf("unable to get subscriptions: %w", err)
	}

	subscription := models.Subscription{
		UserID: userID,
	}

	if err := c.db.First(&subscription, "id = ?", subID).Error; err != nil {
		return subscription, fmt.Errorf("unable to get subscription: %w", err)
	}

	return subscription, nil
}

func (c *Client) AddSubscription(ctx context.Context, item *models.SubscriptionRequest) error {
	userID, err := session.UserID(ctx)
	if err != nil {
		return fmt.Errorf("unable to add subscription: %w", err)
	}

	var (
		feed *models.Feed
		usr  *models.User
		sub  *models.Subscription
	)

	if err = c.db.Transaction(func(tx *gorm.DB) error {
		// Get the user record.
		if err = c.db.First(&usr, "id = ?", userID).Error; err != nil {
			return err
		}

		// Get the feed record with this feed URL.
		err = tx.Where(models.Feed{URL: item.Link}).First(&feed).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// If there is no existing feed record, create a new one.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if feed, err = models.NewFeedFromURL(item.Link); err != nil {
				return err
			}
			if err := tx.Create(&feed).Error; err != nil {
				return err
			}
		}

		// Create a new subscription.
		sub, err = models.NewSubscription(item.Name, feed.ID, userID)
		if err != nil {
			return err
		}

		// Add the subscription to the feed and user records.
		feed.Subscriptions = append(feed.Subscriptions, *sub)
		// usr.Subscriptions = append(usr.Subscriptions, *sub)

		// godump.Dump(feed)

		if err := tx.Save(&feed).Error; err != nil {
			return err
		}
		// Update feed.
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	}); err != nil {
		return fmt.Errorf("failed to add subscription: %w", err)
	}

	return nil
}
