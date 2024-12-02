// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/joshuar/go-feed-me/internal/id"
	"github.com/joshuar/go-feed-me/internal/logging"
)

var (
	ErrAddSubscription     = errors.New("add subscription failed")
	ErrSubscriptionExists  = errors.New("subscription already exists")
	ErrGetSubscriptions    = errors.New("get subscriptions failed")
	ErrSubscriptionInvalid = errors.New("invalid subscription")
)

func AddNewSubscription(ctx context.Context, userID string, cache Cache, db DB, sub *APISubscription) error {
	// Get the feed details by the provided feed URL.
	feed, err := cache.GetFeedByURL(ctx, sub.URL)
	if err != nil {
		return errors.Join(ErrAddSubscription, err)
	}
	// If the feed does not exist, create it.
	if feed.ID == "" {
		newFeed, err := NewFeedFromURL(sub.URL)
		if err != nil {
			return errors.Join(ErrAddSubscription, err)
		}

		logging.FromContext(ctx).Debug("Adding new feed.",
			slog.String("feed_id", feed.ID),
			slog.String("title", feed.Title),
		)

		if err := cache.AddFeeds(ctx, *newFeed); err != nil {
			return errors.Join(ErrAddSubscription, err)
		}

		feed.ID = newFeed.ID
	}

	// Find a subscription by the provided feed URL.
	newSub, err := db.FindSubscriptionByFeedID(ctx, feed.ID)
	if err != nil {
		return errors.Join(ErrAddSubscription, err)
	}
	// If the subscription already exists, nothing to do, exit.
	if newSub.ID != "" {
		return ErrSubscriptionExists
	}
	// Otherwise, create a new subscription from the proivded details.
	newSub, err = newSubscription(sub.Name, feed.ID)
	if err != nil {
		return errors.Join(ErrAddSubscription, err)
	}

	if valid, errs := sub.Valid(ctx); !valid {
		err := ErrSubscriptionInvalid
		for field, problem := range errs {
			return errors.Join(err, fmt.Errorf("%s: %s", field, problem))
		}

		return err
	}

	// Add the new subscription.
	if err := db.AddSubscription(ctx, userID, newSub); err != nil {
		return errors.Join(ErrAddSubscription, err)
	}

	return nil
}

func GetSubcribedFeeds(ctx context.Context, cache Cache, db DB) ([]APIFeed, error) {
	// Get user subscriptions
	subs, err := db.GetAllSubscriptions(ctx)
	if err != nil {
		return nil, errors.Join(ErrGetSubscriptions, err)
	}

	var feedIDs []string
	for _, sub := range subs {
		feedIDs = append(feedIDs, sub.FeedID)
	}

	feeds, err := cache.GetFeeds(ctx, feedIDs...)
	if err != nil {
		return nil, errors.Join(ErrGetSubscriptions, err)
	}

	return feeds, nil
}

func (s *Subscription) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(s)
}

func (s *APISubscription) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(s)
}

func newSubscription(name, feedID string) (*Subscription, error) {
	// If the subscription does not exist
	subID, err := id.NewID(id.Sub)
	if err != nil {
		return nil, errors.Join(ErrInvalidID, err)
	}

	return &Subscription{
			ID:     subID,
			Name:   name,
			FeedID: feedID,
		},
		nil
}
