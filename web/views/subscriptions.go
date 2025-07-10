// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package views

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
)

type SubscriptionsPage struct {
	filters    *models.SubscriptionFilters
	cards      []templ.Component
	categories models.CategoryCounts
}

func NewSubscriptionsPage(filters *models.SubscriptionFilters, categories models.CategoryCounts, cards ...templ.Component) *SubscriptionsPage {
	return &SubscriptionsPage{
		cards:      cards,
		filters:    filters,
		categories: categories,
	}
}
