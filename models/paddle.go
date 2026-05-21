// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "github.com/PaddleHQ/paddle-go-sdk/v5"

func (s *PaddleSubscription) IsActive() bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusActive)
}

func (s *PaddleSubscription) IsTrial() bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionItemStatusTrialing)
}

func (s *PaddleSubscription) IsPastDue() bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusPastDue)
}

func (s *PaddleSubscription) IsPaused() bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusPaused)
}

func (s *PaddleSubscription) IsCancelled() bool {
	return s.SubscriptionStatus == string(paddle.SubscriptionStatusCanceled)
}
