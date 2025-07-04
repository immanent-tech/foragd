// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package views

import (
	"context"
	"slices"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/content"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

type SubscriptionsPage struct {
	filters       *models.SubscriptionFilters
	subscriptions models.Subscriptions
	pagination    models.Pagination
}

func NewSubscriptionsPage(subscriptions models.Subscriptions, filters *models.SubscriptionFilters, pagination models.Pagination) *SubscriptionsPage {
	return &SubscriptionsPage{
		subscriptions: subscriptions,
		filters:       filters,
		pagination:    pagination,
	}
}

func (s SubscriptionsPage) generateCards(ctx context.Context) []templ.Component {
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

func markAllSubscriptionsAction(view models.View) templ.Component {
	parameters := map[string]string{
		"redirect": "/home",
	}
	// Create htmx attributes.
	attrs := templ.Attributes{
		"hx-replace-url": "false",
		"hx-target":      partials.ContentID.Target(),
		"hx-swap":        "innerHTML swap:1s",
		"hx-vals":        partials.GenerateHXVals(parameters),
	}

	switch view {
	case models.ViewUnread:
		attrs["hx-post"] = "/subscriptions/mark/" + string(models.MarkRead)
		return partials.LinkMarkRead("Mark All Read", attrs)
	case models.ViewRead:
		fallthrough
	default:
		attrs["hx-post"] = "/subscriptions/mark/" + string(models.MarkUnread)
		return partials.LinkMarkUnread("Mark All Unread", attrs)
	}
}
