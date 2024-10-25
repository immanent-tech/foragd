// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"fmt"

	"github.com/joshuar/go-feed-me/internal/id"
)

func NewSubscription(name, feedID, userID string) (*Subscription, error) {
	subID, err := id.NewID(id.Sub)
	if err != nil {
		return nil, fmt.Errorf("cannot create subscription: %w", err)
	}

	return &Subscription{
			ID:     subID,
			FeedID: feedID,
			UserID: userID,
			Name:   name,
		},
		nil
}

func (f *SubscriptionRequest) Valid(_ context.Context) (bool, ValidationErrors) {
	return validateStruct(f)
}
