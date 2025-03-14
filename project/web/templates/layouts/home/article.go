// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package home

import (
	"github.com/joshuar/go-feed-me/internal/models"
)

// Article is a display component that shows an article for the given data.
type Article struct {
	*models.APIItem
}

func NewArticle(item *models.APIItem) *Article {
	return &Article{
		APIItem: item,
	}
}
