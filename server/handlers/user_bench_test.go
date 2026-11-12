/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/immanent-tech/foragd/models"
)

func BenchmarkValidateSubscriptionLimits(b *testing.B) {
	mgr := setupManager(b)

	// Set up context.
	ctx := b.Context()
	ctx = models.UserToCtx(ctx, &models.User{})
	userSvc := &MoqUserService{}
	senderSvc := &MoqEmailSender{}

	for size := range slices.Values([]int{10, 100, 250, 1000, models.MaxSubscriptions - 1}) {
		subSvc := &MoqSubscriptionsService{
			GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
				subscriptions := make(models.Subscriptions, 0)
				for range size {
					subscriptions = append(subscriptions, &models.Subscription{
						Type: models.SubscriptionTypeFeed,
					})
				}
				return subscriptions, nil
			},
		}

		mw := mgr.ValidateSubscriptionLimits(userSvc, subSvc, senderSvc)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		b.Run(fmt.Sprintf("subscriptions-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				req := httptest.NewRequest(http.MethodGet, "/articles?sort=NewestFirst&view=Unread&count=10", nil)
				rec := httptest.NewRecorder()

				mw(handler).ServeHTTP(rec, req.WithContext(ctx))
			}
		})
	}
}
