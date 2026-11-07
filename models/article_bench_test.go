/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package models_test

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/immanent-tech/foragd/models"
)

func BenchmarkArticles_GetSubscriptionIDs(b *testing.B) {
	// Generate a set of random subscription IDs.
	subscriptionIDs := make([]models.SubscriptionID, 3000)
	for idx := range subscriptionIDs {
		subscriptionIDs[idx] = gofakeit.ID()
	}

	articles := make(models.Articles, 1000)
	for idx := range articles {
		articles[idx] = &models.Article{
			Subscription: models.Subscription{
				SubscriptionID: strconv.Itoa(rand.IntN(len(subscriptionIDs))),
			},
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		articles.GetSubscriptionIDs()
	}
}

func BenchmarkArticles_GetFeedIDs(b *testing.B) {
	// Generate a set of random subscription IDs.
	feedIDs := make([]models.FeedID, 3000)
	for idx := range feedIDs {
		feedIDs[idx] = gofakeit.ID()
	}

	articles := make(models.Articles, 1000)
	for idx := range articles {
		articles[idx] = &models.Article{

			Item: models.Item{
				FeedID: strconv.Itoa(rand.IntN(len(feedIDs))),
			},
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		articles.GetFeedIDs()
	}
}

func BenchmarkArticles_GetIDs(b *testing.B) {
	articles := make(models.Articles, 1000)
	for idx := range articles {
		articles[idx] = &models.Article{
			Item: models.Item{
				ItemID: gofakeit.ID(),
			},
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		articles.GetIDs()
	}
}
