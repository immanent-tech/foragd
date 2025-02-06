// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "time"

func NewSubscriptionState(details *APISubscriptionRequest) SubscriptionState {
	return SubscriptionState{
		CreatedAt:  time.Now().UTC(),
		Name:       details.Name,
		Categories: details.Categories,
	}
}
