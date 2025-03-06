// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import (
	"time"

	"github.com/joshuar/go-feed-me/internal/models"
)

func NewSubscriptionState(details *SubscriptionRequest) models.SubscriptionState {
	return models.SubscriptionState{
		CreatedAt:  time.Now().UTC(),
		Name:       details.Name,
		Categories: details.Categories,
	}
}
