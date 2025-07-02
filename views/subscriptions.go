// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package views

import (
	"context"
	"slices"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/content"
)

type Subscriptions struct {
	filters       *models.SubscriptionFilters
	subscriptions models.Subscriptions
	pagination    models.Pagination
}

func NewSubscriptions(subscriptions models.Subscriptions, filters *models.SubscriptionFilters, pagination models.Pagination) *Subscriptions {
	return &Subscriptions{
		subscriptions: subscriptions,
		filters:       filters,
		pagination:    pagination,
	}
}

func (s Subscriptions) generateCards(ctx context.Context) []templ.Component {
	cards := make([]templ.Component, 0, len(s.subscriptions))
	for subscription := range slices.Values(s.subscriptions) {
		cards = append(cards, content.NewSubscriptionContent(subscription).Card())
	}
	// Add pagination element if pagination is required.
	if s.pagination != "" && len(cards) == s.filters.GetCount() {
		// Add pagination htmx props to last article.
		cards = append(cards, content.PaginationControl(ctx, "/subscriptions", s.pagination))
	}
	return cards
}
