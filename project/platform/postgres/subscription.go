// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/model"
	"github.com/joshuar/go-feed-me/platform/id"
)

func (c *Client) GetUserSubscriptions(ctx context.Context, item *model.Subscription) error {
	return nil
}

func (c *Client) AddSubscription(ctx context.Context, item *model.Subscription) error {
	user, err := c.GetUser(ctx)
	if err != nil {
		return fmt.Errorf("unable to add subscription: %w", err)
	}

	var feed *Feed

	if err = c.db.Transaction(func(tx *gorm.DB) error {
		// Get any existing feed, or, create a new feed.
		if feed, err = c.getFeedByURL(item.Link); err != nil {
			feed, err = newFeedRecord(item.Link)
			if err != nil {
				return err
			}
		}

		// Create a new subscription.
		subscription, err := newSubscriptionRecord(item.Name)
		if err != nil {
			return err
		}
		// Add the feed and user IDs.
		subscription.FeedID = feed.ID
		subscription.UserID = user.UserID()
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

func newSubscriptionRecord(name string) (*Subscription, error) {
	subID, err := id.NewID(id.Sub)
	if err != nil {
		return nil, fmt.Errorf("cannot create subscription: %w", err)
	}

	return &Subscription{
			ID:   subID,
			Name: name,
		},
		nil
}
