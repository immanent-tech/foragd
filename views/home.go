// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package views

import (
	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
)

type HomePage struct {
	Subscriptions  models.Subscriptions
	TopCategories  templ.Component
	RareCategories templ.Component
	RandomArticles []templ.Component
	Theme          string
}
