// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package views

import (
	"context"
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/models"
	"github.com/joshuar/go-feed-me/web/templates/content"
	"github.com/joshuar/go-feed-me/web/templates/partials"
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
		cards = append(cards, NewArticleContent(article).Card())
	}
	// Add pagination element if pagination is required.
	if s.pagination != "" && len(cards) == s.filters.GetCount() {
		// Add pagination htmx props to last article.
		cards = append(cards, content.PaginationControl(ctx, "/articles", s.pagination))
	}
	return cards
}

// MarkAllItemsAction generates an action for the footer menu to mark all items of a feed as either read or unread,
// depending on the current view.
func markAllArticles(view models.View, subIDs ...models.SubscriptionID) templ.Component {
	// Create url parameters.
	parameters := map[string]string{
		models.ParamSubscriptions: strings.Join(subIDs, ","),
		"redirect":                "/subscriptions",
	}
	// Create htmx attributes.
	attrs := templ.Attributes{
		"hx-swap":        "innerHTML swap:1s",
		"hx-vals":        partials.GenerateHXVals(parameters),
	}
	switch view {
	case models.ViewUnread:
		attrs["hx-post"] = "/subscriptions/mark/" + string(models.MarkRead)
		return partials.LinkMarkRead("Mark All Read", attrs)
	case models.ViewRead:
		attrs["hx-post"] = "/subscriptions/mark/" + string(models.MarkUnread)
		return partials.LinkMarkUnread("Mark All Unread", attrs)
	default:
		return nil
	}
}
