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

type ArticlesPage struct {
	filters    *models.ArticleFilters
	articles   models.Articles
	pagination models.Pagination
}

func NewArticlesPage(articles models.Articles, filters *models.ArticleFilters, pagination models.Pagination) *ArticlesPage {
	return &ArticlesPage{
		articles:   articles,
		filters:    filters,
		pagination: pagination,
	}
}

func (s ArticlesPage) generateCards(ctx context.Context) []templ.Component {
	cards := make([]templ.Component, 0, len(s.articles))
	for article := range slices.Values(s.articles) {
		cards = append(cards, NewArticleContent(article).ShowAsCard())
	}
	// Add pagination element if pagination is required.
	if s.pagination != "" && len(cards) == s.filters.GetCount() {
		// Add pagination htmx props to last article.
		cards = append(cards, content.PaginationControl(ctx, "/articles", s.pagination))
	}
	return cards
}
