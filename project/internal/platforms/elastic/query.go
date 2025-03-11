// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"reflect"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/internal/models"
)

// QueryOption is a functional option for queries.
type QueryOption Option[*types.Query]

// NumberRangeQueryOption is a functional option for a number range query.
type NumberRangeQueryOption Option[*types.NumberRangeQuery]

// BoolQueryOption is a functional option for a boolean query.
type BoolQueryOption Option[*types.BoolQuery]

// QueryMatchAll adds a "Match All" clause.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
func QueryMatchAll() QueryOption {
	return func(query *types.Query) {
		query.MatchAll = types.NewMatchAllQuery()
	}
}

// QueryByTerm adds a "Term" query on the given field with the given value.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
func QueryByTerm(field string, value any) QueryOption {
	return func(query *types.Query) {
		if value != nil {
			query.Term = map[string]types.TermQuery{
				field: {Value: value},
			}
		}
	}
}

// QueryByFeedIDs adds a "Terms" clause with the given Feed IDs.
func QueryByFeedIDs(feedIDs ...models.FeedID) QueryOption {
	return func(query *types.Query) {
		if len(feedIDs) > 0 {
			query.Terms = &types.TermsQuery{
				TermsQuery: map[string]types.TermsQueryField{
					"feed_id": feedIDs,
				},
			}
		}
	}
}

// QueryByItemIDs adds a "Terms" clause with the given Item IDs.
func QueryByItemIDs(itemIDs ...models.ItemID) QueryOption {
	return func(query *types.Query) {
		if len(itemIDs) > 0 {
			query.Terms = &types.TermsQuery{
				TermsQuery: map[string]types.TermsQueryField{
					"item_id": itemIDs,
				},
			}
		}
	}
}

// QueryByURLs adds a "Terms" clause to query the given field with a list of URLs.
func QueryByURLs(field string, urls ...string) QueryOption {
	return func(query *types.Query) {
		if len(urls) > 0 {
			query.Terms = &types.TermsQuery{
				TermsQuery: map[string]types.TermsQueryField{
					field: urls,
				},
			}
		}
	}
}

// QueryByCategory adds a "Terms" clause to query by the given list of category names.
func QueryByCategory(categories ...models.Category) QueryOption {
	return func(query *types.Query) {
		if len(categories) > 0 {
			query.Terms = &types.TermsQuery{
				TermsQuery: map[string]types.TermsQueryField{
					"categories.raw": categories,
				},
			}
		}
	}
}

// QuerySince adds a "Range" query to find documents newer than the given time.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func QuerySince(field string, since time.Time) QueryOption {
	return func(query *types.Query) {
		var sinceStr string
		if since.IsZero() {
			sinceStr = "0"
		} else {
			sinceStr = since.UTC().Format(time.RFC3339Nano)
		}

		query.Range = map[string]types.RangeQuery{
			field: types.DateRangeQuery{
				Gte: &sinceStr,
			},
		}
	}
}

// QuerySince adds a "Range" query to find documents between (or equal to) the given times.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func QueryBetween(field string, from time.Time, to time.Time) QueryOption {
	return func(query *types.Query) {
		var fromStr, toStr string

		if !from.IsZero() && !to.IsZero() {
			fromStr = from.UTC().Format(time.RFC3339Nano)
			toStr = to.UTC().Format(time.RFC3339Nano)

			query.Range = map[string]types.RangeQuery{
				field: types.DateRangeQuery{
					Gte: &fromStr,
					Lte: &toStr,
				},
			}
		}
	}
}

func IntLessThan(value int64) Option[*types.NumberRangeQuery] {
	return func(numberRange *types.NumberRangeQuery) {
		lt := types.Float64(value)
		numberRange.Lt = &lt
	}
}

// QueryBool constructs a bool query with the given query options and adds it to
// the query.
func QueryNumberRange(field string, options ...NumberRangeQueryOption) QueryOption {
	return func(query *types.Query) {
		rangeQuery := &types.NumberRangeQuery{}

		for _, option := range options {
			option(rangeQuery)
		}

		if !reflect.DeepEqual(rangeQuery, &types.NumberRangeQuery{}) {
			if query.Range == nil {
				query.Range = make(map[string]types.RangeQuery)
			}

			query.Range[field] = rangeQuery
		}
	}
}

// BoolFilter sets the given query options as the "filter" clause of the bool query.
func BoolFilter(queryOptions ...QueryOption) BoolQueryOption {
	return func(boolQueryClause *types.BoolQuery) {
		var filters []types.Query
		// Create queries for each of the passed in query options and append to
		// the bool filters list.
		for _, queryOption := range queryOptions {
			filterClause := &types.Query{}
			queryOption(filterClause)

			if !reflect.DeepEqual(filterClause, &types.Query{}) {
				filters = append(filters, *filterClause)
			}
		}

		// Create the filter clause.
		if len(filters) > 0 {
			boolQueryClause.Filter = filters
		}
	}
}

// BoolMust sets the given query options as the "must" clause of the bool query.
func BoolMust(queryOptions ...QueryOption) BoolQueryOption {
	return func(boolQueryClause *types.BoolQuery) {
		var musts []types.Query
		// Create queries for each of the passed in query options and append to
		// the bool must list.
		for _, queryOption := range queryOptions {
			mustClause := &types.Query{}
			queryOption(mustClause)

			if !reflect.DeepEqual(mustClause, &types.Query{}) {
				musts = append(musts, *mustClause)
			}
		}
		// Create the must clause.
		if len(musts) > 0 {
			boolQueryClause.Must = musts
		}
	}
}

// BoolMustNot sets the given query options as the "must_not" clause of the bool query.
func BoolMustNot(queryOptions ...QueryOption) BoolQueryOption {
	return func(boolQueryClause *types.BoolQuery) {
		var mustNots []types.Query
		// Create queries for each of the passed in query options and append to
		// the bool must_not list.
		for _, queryOption := range queryOptions {
			mustNotClause := &types.Query{}
			queryOption(mustNotClause)

			if !reflect.DeepEqual(mustNotClause, &types.Query{}) {
				mustNots = append(mustNots, *mustNotClause)
			}
		}
		// Create the must_not clause.
		if len(mustNots) > 0 {
			boolQueryClause.MustNot = mustNots
		}
	}
}

// BoolMustNot sets the given query options as the "must_not" clause of the bool query.
func BoolShould(queryOptions ...QueryOption) BoolQueryOption {
	return func(boolQueryClause *types.BoolQuery) {
		var shoulds []types.Query
		// Create queries for each of the passed in query options and append to
		// the bool should list.
		for _, queryOption := range queryOptions {
			shouldClause := &types.Query{}
			queryOption(shouldClause)

			if !reflect.DeepEqual(shouldClause, &types.Query{}) {
				shoulds = append(shoulds, *shouldClause)
			}
		}
		// Create the must_not clause.
		if len(shoulds) > 0 {
			boolQueryClause.Should = shoulds
		}
	}
}

// QueryBool constructs a bool query with the given query options and adds it to
// the query.
func QueryBool(options ...BoolQueryOption) QueryOption {
	return func(query *types.Query) {
		boolQuery := &types.BoolQuery{}

		for _, option := range options {
			option(boolQuery)
		}

		if !reflect.DeepEqual(boolQuery, &types.BoolQuery{}) {
			query.Bool = boolQuery
		}
	}
}

func BuildQuery(options ...QueryOption) *types.Query {
	queryOptions := &types.Query{}

	for _, option := range options {
		option(queryOptions)
	}

	if !reflect.DeepEqual(queryOptions, &types.Query{}) {
		return queryOptions
	}

	return nil
}
