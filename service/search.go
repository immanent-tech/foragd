// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/textquerytype"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/elastic/query"
)

// BuildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func BuildSearchResultsQuery(
	ctx context.Context,
	user *models.User,
	request *models.SearchRequest,
	clause query.BoolOption,
) (query.Option, error) {
	var (
		loc *time.Location
		err error
	)
	if request.Timezone != "" {
		loc, err = time.LoadLocation(request.Timezone)
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	} else {
		loc, err = time.LoadLocation("UTC")
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	}
	var (
		since time.Time
		pivot string
	)
	switch request.PublishedWithin {
	case models.SearchRequestPublishedWithinLastHour:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-time.Hour).Format(time.Layout), loc)
		pivot = "30m"
	case models.SearchRequestPublishedWithinLast12hours:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-12*time.Hour).Format(time.Layout), loc)
		pivot = "6h"
	case models.SearchRequestPublishedWithinLastDay:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-24*time.Hour).Format(time.Layout), loc)
		pivot = "12h"
	case models.SearchRequestPublishedWithinLastWeek:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
		pivot = "3d"
	case models.SearchRequestPublishedWithinLastMonth:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-30*24*time.Hour).Format(time.Layout), loc)
		pivot = "14d"
	default:
		pivot = "3d"
	}

	// Get subscriptions.
	var subscriptions models.Subscriptions
	if len(request.Subscriptions) > 0 {
		subscriptions, err = GetSubscriptionsByID(ctx, request.Subscriptions...)
	} else {
		subscriptions, err = GetAllSubscriptions(ctx)
	}
	switch {
	case err != nil:
		return nil, fmt.Errorf("get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("get subscriptions: %w", models.ErrNotFound)
	}

	return query.Bool(
		query.WithBoolQueryName("search-results"),
		query.Filter(
			// Must be in the given user subscriptions.
			query.Bool(
				query.Should(BuildItemQueries(user, request.View, subscriptions)...),
			),
			// Must be published/updated since the given time.
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
				),
			),
		),
		query.Should(
			// Boost items that are from a favorite subscription.
			query.Terms(
				"feed_id",
				subscriptions.FilterByFavorites(true).GetFeedIDs(),
				query.WithQueryName[*query.TermsQuery]("boost-favorites"),
				query.WithQueryBoost[*query.TermsQuery](2.0),
			),
			// Boost documents closer to the current time.
			query.Distance("published", pivot, "now"),
			query.Distance("updated", pivot, "now"),
		),
		clause,
	), nil
}

func StandardSearchResultsClause(search *models.SearchRequest) query.BoolOption {
	// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
	// search suggestion).
	return query.Must(
		// Search across title, description and content fields, with preference for match in that order (via field
		// boosting).
		query.Bool(
			query.Should(
				query.Term("title.exact", search.Text, query.WithQueryBoost[*query.TermQuery](10.0)),
				query.SimpleQueryString(
					query.WithSimpleQueryStringText(&search.Text),
					query.WithSimpleQueryStringFields("title^6", "description^3", "content"),
					query.WithSimpleQueryStringOperator(&operator.And),
				),
				query.MultiMatch(
					search.Text,
					[]string{"description^3", "content"},
					query.WithTextQueryType(textquerytype.Phrase),
				),
			),
		),
		// Search in categories.
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(search.Categories),
			query.WithSimpleQueryStringFields("categories"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		// Search in authors, contributors.
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(search.Authors),
			query.WithSimpleQueryStringFields("authors", "contributors"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
	)
}

func SemanticSearchResultsClause(search *models.SearchRequest) query.BoolOption {
	return query.Must(
		// Perform semantic search on content field for text.
		query.Match("content_semantic", search.Text),
		// Search in categories.
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(search.Categories),
			query.WithSimpleQueryStringFields("categories"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
		// Search in authors, contributors.
		query.SimpleQueryString(
			query.WithSimpleQueryStringText(search.Authors),
			query.WithSimpleQueryStringFields("authors", "contributors"),
			query.WithSimpleQueryStringOperator(&operator.And),
		),
	)
}

func SearchSuggestionsClause(search *models.SearchRequest) query.BoolOption {
	// Must match at least one of in title, description, content.
	return query.Must(
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
	)

}
