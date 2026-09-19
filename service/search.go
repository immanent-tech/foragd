// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// BuildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func BuildSearchResultsQuery(
	ctx context.Context,
	user *models.User,
	request *models.SearchRequest,
	clause query.Option,
) (query.Option, error) {
	// Get user subscriptions.
	allSubscriptions := models.SubscriptionsFromCtx(ctx)
	if len(allSubscriptions) == 0 {
		return nil, fmt.Errorf("get user subscriptions: %w", models.ErrCtxValueNotFound)
	}
	subscriptions := allSubscriptions.FilterByIDs(request.Subscriptions...)

	return query.Bool(
		query.Must(
			query.Bool(
				query.WithBoolQueryName("search-filters"),
				query.Filter(
					query.Bool(
						// Must satisfy user global filters.
						ArticleFiltersQueryClause(user.GetSettings().GlobalFilters),
						// Must be in the given user subscriptions.
						query.Should(BuildItemQueries(user, request.View, subscriptions)...),
					),
					// Must be published/updated since the given time.
					query.Bool(
						query.Should(
							query.Since("published", request.Since()),
							query.Since("updated", request.Since()),
						),
					),
				),
				query.Should(
					// Boost items that are from a favorite subscription.
					query.Terms(
						"feed_id",
						subscriptions.FilterByView(models.ViewFavorites).GetFeedIDs(),
						query.WithQueryName[*query.TermsQuery]("boost-favorites"),
						query.WithQueryBoost[*query.TermsQuery](2.0),
					),
					// Boost documents closer to the current time.
					query.Distance("published", request.Pivot(), "now"),
					query.Distance("updated", request.Pivot(), "now"),
				),
			),
			clause,
		),
	), nil
}

func StandardSearchResultsClause(search *models.SearchRequest) query.Option {
	// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
	// search suggestion).
	return query.Bool(
		query.Must(
			query.Bool(
				query.Should(
					// Boost match title exactly.
					query.Term("title.exact", search.Text, query.WithQueryBoost[*query.TermQuery](10.0)),
					// Simple query string across most fields. Boost title and description matches.
					query.SimpleQueryString(
						query.WithSimpleQueryStringText(&search.Text),
						query.WithSimpleQueryStringFields(
							"title^3",
							"description^2",
							"content",
							"categories",
							"authors",
							"contributors",
						),
						query.WithSimpleQueryStringOperator(&operator.And),
					),
				),
			),
		),
	)
}

func SemanticSearchResultsClause(search *models.SearchRequest) query.Option {
	// Perform semantic search on content field for text.
	return query.Match("content_semantic", search.Text)
}

func SearchSuggestionsClause(search *models.SearchRequest) query.Option {
	// Must match at least one of in title, description, content.
	return query.Bool(
		query.Must(
			query.Bool(
				query.Should(
					query.Term("title.exact", search.Text, query.WithQueryBoost[*query.TermQuery](10.0)),
					query.SearchAsYouType(search.Text, "title"),
					query.SearchAsYouType(search.Text, "description"),
					query.SimpleQueryString(
						query.WithSimpleQueryStringText(&search.Text),
						query.WithSimpleQueryStringFields("content"),
						query.WithSimpleQueryStringOperator(&operator.And),
					),
				),
			),
		),
	)
}
