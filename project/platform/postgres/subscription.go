// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/joshuar/go-feed-me/model"
	"github.com/joshuar/go-feed-me/platform/id"
)

func (c *Client) AddSubscription(ctx context.Context, item *model.Subscription) error {
	user, err := c.GetUser(ctx)
	if err != nil {
		return fmt.Errorf("unable to add subscription: %w", err)
	}

	var feed *Feed

	return c.db.Transaction(func(tx *gorm.DB) error {
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
	})
}

func newSubscriptionRecord(name string) (*Subscription, error) {
	if subID, err := id.NewID(id.Sub); err != nil {
		return nil, fmt.Errorf("cannot create subscription: %w", err)
	} else {
		return &Subscription{
				ID:   subID,
				Name: name,
			},
			nil
	}
}
