/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models_test

import (
	"math/rand/v2"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"

	"github.com/immanent-tech/foragd/models"
)

func TestSubscriptions_FilterByIDs(t *testing.T) {
	wanted := make(models.Subscriptions, 0, 5)
	for range 5 {
		wanted = append(wanted, &models.Subscription{SubscriptionID: gofakeit.ID()})
	}
	subscriptions := make(models.Subscriptions, 0)
	for range models.MaxSubscriptions - 5 {
		subscriptions = append(subscriptions, &models.Subscription{SubscriptionID: gofakeit.ID()})
	}
	subscriptions = append(subscriptions, wanted...)

	tests := []struct {
		name string
		ids  []models.SubscriptionID
		want models.Subscriptions
	}{
		{
			name: "ok",
			ids:  wanted.GetIDs(),
			want: wanted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subscriptions.FilterByIDs(tt.ids...)
			assert.EqualValues(t, wanted, got)
		})
	}
}

func TestSubscriptions_FilterByFeedIDs(t *testing.T) {
	wanted := make(models.Subscriptions, 0, 5)
	for range 5 {
		wanted = append(
			wanted,
			&models.Subscription{Type: models.SubscriptionTypeFeed, FeedData: &models.FeedSubscription{
				FeedID: gofakeit.ID(),
			}},
		)
	}
	subscriptions := make(models.Subscriptions, 0)
	for range models.MaxSubscriptions - 5 {
		subscriptions = append(subscriptions,
			&models.Subscription{Type: models.SubscriptionTypeFeed, FeedData: &models.FeedSubscription{
				FeedID: gofakeit.ID(),
			}},
		)
	}
	subscriptions = append(subscriptions, wanted...)

	tests := []struct {
		name string
		ids  []models.SubscriptionID
		want models.Subscriptions
	}{
		{
			name: "ok",
			ids:  wanted.GetFeedIDs(),
			want: wanted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subscriptions.FilterByFeedIDs(tt.ids...)
			assert.EqualValues(t, wanted, got)
		})
	}
}

func TestSubscriptions_FilterByCategories(t *testing.T) {
	wantedCategories := []models.Category{gofakeit.BuzzWord()}
	wanted := models.Subscriptions{
		&models.Subscription{SubscriptionID: gofakeit.ID(),
			Customisation: models.SubscriptionCustomisation{
				Categories: []models.Category{wantedCategories[0]},
			}},
	}
	subscriptions := make(models.Subscriptions, 0)
	for range 5 {
		s := &models.Subscription{SubscriptionID: gofakeit.ID(),
			Customisation: models.SubscriptionCustomisation{
				Categories: []models.Category{gofakeit.BuzzWord()},
			}}
		subscriptions = append(subscriptions, s)
	}
	subscriptions[rand.IntN(5)] = wanted[0]

	tests := []struct {
		name       string
		categories []models.Category
		want       models.Subscriptions
	}{
		{
			name:       "ok",
			categories: wantedCategories,
			want:       wanted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := subscriptions.FilterByCategories(tt.categories...)
			assert.EqualValues(t, wanted, got)
		})
	}
}
