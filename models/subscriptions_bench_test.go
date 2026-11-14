/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/immanent-tech/foragd/models"
)

func BenchmarkFilterByIDs(b *testing.B) {
	wantedID := "wanted"
	for size := range slices.Values([]int{10, 100, 250, 1000, models.MaxSubscriptions - 1}) {
		subscriptions := make(models.Subscriptions, 0)
		for range size {
			subscriptions = append(subscriptions, &models.Subscription{SubscriptionID: gofakeit.ID()})
		}
		subscriptions[rand.IntN(size)] = &models.Subscription{SubscriptionID: wantedID}

		b.Run(fmt.Sprintf("subscriptions-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				subscriptions.FilterByIDs(wantedID)
			}
		})
	}
}

func BenchmarkFilterByFeedIDs(b *testing.B) {
	wantedID := "wanted"
	for size := range slices.Values([]int{10, 100, 250, 1000, models.MaxSubscriptions - 1}) {
		subscriptions := make(models.Subscriptions, 0)
		for range size {
			subscriptions = append(
				subscriptions,
				&models.Subscription{Type: models.SubscriptionTypeFeed, FeedData: &models.FeedSubscription{
					FeedID: gofakeit.ID(),
				}},
			)
		}
		subscriptions[rand.IntN(size)] = &models.Subscription{
			Type: models.SubscriptionTypeFeed,
			FeedData: &models.FeedSubscription{
				FeedID: wantedID,
			},
		}

		b.Run(fmt.Sprintf("subscriptions-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				subscriptions.FilterByFeedIDs(wantedID)
			}
		})
	}
}

func BenchmarkExcludeIDs(b *testing.B) {
	wantedID := "wanted"
	for size := range slices.Values([]int{10, 100, 250, 1000, models.MaxSubscriptions - 1}) {
		subscriptions := make(models.Subscriptions, 0)
		for range size {
			subscriptions = append(subscriptions, &models.Subscription{SubscriptionID: gofakeit.ID()})
		}
		subscriptions[rand.IntN(size)] = &models.Subscription{SubscriptionID: wantedID}

		b.Run(fmt.Sprintf("subscriptions-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				subscriptions.ExcludeIDs(wantedID)
			}
		})
	}
}

func BenchmarkFilterByCategories(b *testing.B) {
	wanted := gofakeit.BuzzWord()
	for size := range slices.Values([]int{10, 100, 250, 1000, models.MaxSubscriptions - 1}) {
		subscriptions := make(models.Subscriptions, 0)
		for range size {
			s := &models.Subscription{SubscriptionID: gofakeit.ID(),
				Customisation: models.SubscriptionCustomisation{
					Categories: []models.Category{gofakeit.BuzzWord()},
				}}
			subscriptions = append(subscriptions, s)
		}
		subscriptions[rand.IntN(size)] = &models.Subscription{SubscriptionID: gofakeit.ID(),
			Customisation: models.SubscriptionCustomisation{
				Categories: []models.Category{wanted},
			}}

		b.Run(fmt.Sprintf("subscriptions-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				subscriptions.FilterByCategories(wanted)
			}
		})
	}
}
