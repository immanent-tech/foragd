// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/joshuar/go-feed-me/internal/id"
)

var (
	ErrAddSubscription     = errors.New("add subscription failed")
	ErrSubscriptionExists  = errors.New("subscription already exists")
	ErrGetSubscriptions    = errors.New("get subscriptions failed")
	ErrSubscriptionInvalid = errors.New("invalid subscription")
)

func NewSubscription(ctx context.Context, cache Cache, db DB, newSub *APISubscription) error {
	sub, err := db.GetSubscriptionByURL(ctx, newSub.URL)
	if err != nil {
		return errors.Join(ErrAddSubscription, err)
	}

	if sub.ID != "" {
		return ErrSubscriptionExists
	}

	subID, err := id.NewID(id.Sub)
	if err != nil {
		return errors.Join(ErrInvalidID, err)
	}

	sub.ID = subID
	sub.Name = newSub.Name

	feed, err := cache.GetFeedByURL(ctx, sub.URL)
	if err != nil {
		return errors.Join(ErrAddSubscription, err)
	}

	if feed.ID == "" {
		feed, err := NewFeedFromURL(sub.URL)
		if err != nil {
			return errors.Join(ErrAddSubscription, err)
		}

		if err := cache.AddFeed(ctx, *feed); err != nil {
			return errors.Join(ErrAddSubscription, err)
		}

		sub.FeedID = feed.ID
	}

	if valid, errs := sub.Valid(ctx); !valid {
		err := ErrSubscriptionInvalid
		for field, problem := range errs {
			return errors.Join(err, fmt.Errorf("%s: %s", field, problem))
		}

		return err
	}

	if err := db.AddSubscription(ctx, &sub); err != nil {
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
