// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/davecgh/go-spew/spew"

	"github.com/joshuar/go-feed-me/internal/models"
)

var ErrSaveSubscription = errors.New("save subscription failed")

// AddSubscriptions will add the given subscriptions to the user's subscriptions.
func (c *Client) AddSubscriptions(ctx context.Context, subscriptions ...*models.APISubscriptionRequest) error {
	var warnings error

	user, found := models.UserFromCtx(ctx)
	if !found {
		return ErrNoUserCtx
	}

	// Gather all the URLs for the subscriptions.
	urls := make([]models.URL, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		urls = append(urls, subscription.URL)
	}

	// Get a list of existing subscribedFeeds by the subscription URLs.
	feeds, err := c.GetFeedsByURL(ctx, urls...)
	if err != nil {
		return errors.Join(ErrSaveSubscription, err)
	}

	for _, subscription := range subscriptions {
		var feedID models.FeedID
		// Check if there is an existing feed for the new subscription.
		idx := slices.IndexFunc(feeds, func(feed models.APIFeed) bool { return feed.FeedURL == subscription.URL })
		if idx > 0 {
			feedID = feeds[idx].ID
			// For any existing feeds, ignore if the user is already subscribed.
			if user.IsSubscribed(feedID) {
				warnings = errors.Join(warnings, fmt.Errorf("%w: %s", models.ErrUserAlreadySubscribed, subscription.URL))
				continue
			}
			slog.Info("existing feed")
			spew.Dump(feeds[idx])
		}
		// If there is no existing feed, create a new feed.
		if idx == -1 {
			newFeed, err := models.NewFeedFromURL(ctx, subscription.URL)
			if err != nil {
				warnings = errors.Join(warnings, err)
				continue
			}

			// Add the feed.
			// if err = c.AddFeeds(ctx, *newFeed); err != nil {
			// 	warnings = errors.Join(warnings, err)
			// 	continue
			// }
			slog.Info("new feed")
			spew.Dump(newFeed)
			feedID = newFeed.ID
		}
		// Create a new subscription.
		slog.Info("new subscription")
		spew.Dump(subscription)

		// if err = user.AddSubscription(feedID, *subscription.Name, subscription.Categories); err != nil {
		// 	if !errors.Is(err, models.ErrUserAlreadySubscribed) {
		// 		warnings = errors.Join(ErrSaveSubscription, err)
		// 	}
		// }
	}

	// if err := c.userActionUpdateSubscriptions(ctx, user.ID, user.Subscriptions); err != nil {
	// 	warnings = errors.Join(ErrSaveSubscription, err)
	// }

	return warnings
}
