// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import (
	"time"

	"github.com/joshuar/go-feed-me/internal/models"
)

func NewSubscriptionState(details *SubscriptionRequest) models.SubscriptionState {
	req := models.SubscriptionState{
		CreatedAt: time.Now().UTC(),
	}
	if details.Name != nil {
		req.Name = *details.Name
	}
	if details.Categories != nil {
		// if len(details.Categories) > 0 {
		req.Categories = details.Categories
	}
	return req
}
