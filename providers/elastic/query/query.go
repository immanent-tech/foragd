// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package query contains methods for building Elasticsearch queries.
package query

import (
	"reflect"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"

	"github.com/joshuar/go-feed-me/models"
)

// Option is a functional option for queries.
type Option func(*types.Query)

// NumberRangeOption is a functional option for a number range query.
type NumberRangeOption func(*types.NumberRangeQuery)

// BoolOption is a functional option for a boolean query.
type BoolOption func(*types.BoolQuery)

// MatchAll adds a "Match All" clause.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
func MatchAll() Option {
	return func(query *types.Query) {
		query.MatchAll = types.NewMatchAllQuery()
	}
}

// Term adds a "Term" query on the given field with the given value.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
func Term(field string, value any) Option {
	return func(query *types.Query) {
		if value != nil {
			query.Term = map[string]types.TermQuery{
				field: {Value: value},
			}
		}
	}
}

// User adds a "Term" query to search for docs with the given User ID.
func User(user models.UserID) Option {
	return func(query *types.Query) {
		if user != "" {
			query.Term = map[string]types.TermQuery{
				"user_id": {Value: user},
			}
		}
	}
}

// FeedIDs adds a "Terms" clause with the given Feed IDs.
func FeedIDs(feedIDs ...models.FeedID) Option {
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

// ItemIDs adds a "Terms" clause with the given Item IDs.
func ItemIDs(itemIDs ...models.ItemID) Option {
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

// URLs adds a "Terms" clause to query the given field with a list of URLs.
func URLs(field string, urls ...string) Option {
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

// Categories adds a "Terms" clause to query by the given list of category names.
func Categories(categories ...models.Category) Option {
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

// Since adds a "Range" query to find documents newer than the given time.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func Since(field string, since time.Time) Option {
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

// Between adds a "Range" query to find documents between (or equal to) the given times.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func Between(field string, from time.Time, to time.Time) Option {
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

// IntLessThan creates a range option to retrieve documents with the field value less than the given int.
func IntLessThan(value int64) NumberRangeOption {
	return func(numberRange *types.NumberRangeQuery) {
		lt := types.Float64(value)
		numberRange.Lt = &lt
	}
}

// NumberRange constructs a bool query with the given query options and adds it to
// the query.
func NumberRange(field string, options ...NumberRangeOption) Option {
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

// Filter sets the given query options as the "filter" clause of the bool query.
func Filter(queryOptions ...Option) BoolOption {
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

// Must sets the given query options as the "must" clause of the bool query.
func Must(queryOptions ...Option) BoolOption {
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

// MustNot sets the given query options as the "must_not" clause of the bool query.
func MustNot(queryOptions ...Option) BoolOption {
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

// Should sets the given query options as the "should" clause of the bool query.
func Should(queryOptions ...Option) BoolOption {
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

// BoolQueryName assigns the given string as the name of the query, which allows
// tracking when this bool clause matches documents.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-bool-query.html#named-queries
func BoolQueryName(name string) BoolOption {
	return func(boolQueryClause *types.BoolQuery) {
		if name != "" {
			boolQueryClause.QueryName_ = &name
		}
	}
}

// Bool constructs a bool query with the given query options and adds it to
// the query.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-bool-query.html
func Bool(options ...BoolOption) Option {
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

// Build creates a query from the given options.
func Build(options ...Option) *types.Query {
	queryOptions := &types.Query{}

	for _, option := range options {
		option(queryOptions)
	}

	if !reflect.DeepEqual(queryOptions, &types.Query{}) {
		return queryOptions
	}

	return nil
}
