// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package views

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/partials"
)

type SubscriptionsPage struct {
	filters    *models.SubscriptionFilters
	cards      []templ.Component
	categories models.CategoryCounts
}

func NewSubscriptionsPage(filters *models.SubscriptionFilters, cards ...templ.Component) *SubscriptionsPage {
	return &SubscriptionsPage{
		cards:   cards,
		filters: filters,
	}
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
