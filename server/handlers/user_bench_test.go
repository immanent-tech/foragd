/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/immanent-tech/foragd/models"
)

func BenchmarkValidateSubscriptionLimits_HighSubscriptionCount(b *testing.B) {
	mgr := setupManager(b)

	// Set up context.
	ctx := b.Context()
	ctx = models.UserToCtx(ctx, &models.User{})
	subSvc := &MoqSubscriptionsService{
		GetAllSubscriptionsFunc: func(ctx context.Context) (models.Subscriptions, error) {
			subscriptions := make(models.Subscriptions, 0)
			for range models.MaxSubscriptions - 1 {
				subscriptions = append(subscriptions, &models.Subscription{
					Type: models.SubscriptionTypeFeed,
				})
			}
			return subscriptions, nil
		},
	}
	userSvc := &MoqUserService{}
	senderSvc := &MoqEmailSender{}
	mw := mgr.ValidateSubscriptionLimits(userSvc, subSvc, senderSvc)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	b.ReportAllocs()
	for b.Loop() {
		req := httptest.NewRequest(http.MethodGet, "/articles?sort=NewestFirst&view=Unread&count=10", nil)
		rec := httptest.NewRecorder()

		mw(handler).ServeHTTP(rec, req.WithContext(ctx))
	}
}
