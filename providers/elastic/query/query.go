// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package query contains methods for building Elasticsearch queries.
package query

import (
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/textquerytype"
)

const FuzzinessAuto = "AUTO"

// Option is a functional option for queries.
type Option func(*types.Query)

// NumberRangeOption is a functional option for a number range query.
type NumberRangeOption func(*types.NumberRangeQuery)

// MatchAll adds a "Match All" clause.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
func MatchAll() Option {
	return func(query *types.Query) {
		query.MatchAll = types.NewMatchAllQuery()
	}
}

// Exists query adds an "Exists" clause.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-exists-query
func Exists(field string) Option {
	return func(q *types.Query) {
		name := "exists-" + field
		q.Exists = types.NewExistsQuery()
		q.Exists.Field = field
		q.Exists.QueryName_ = &name
	}
}

// Match adds a "Match" query on the given field with the given value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-match-query
func Match(field, fuzziness string, value string) Option {
	return func(query *types.Query) {
		if value != "" {
			name := "match-" + field
			matchQuery := types.MatchQuery{
				Query:      value,
				QueryName_: &name,
			}
			if fuzziness != "" {
				matchQuery.Fuzziness = fuzziness
			}
			query.Match = map[string]types.MatchQuery{
				field: matchQuery,
			}
		}
	}
}

// MultiMatch adds a "MultiMatch" query on the given field with the given value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-multi-match-query
func MultiMatch(value, fuzziness string, fields ...string) Option {
	return func(query *types.Query) {
		if value != "" {
			name := "multi-match-" + strings.Join(fields, "+")
			query.MultiMatch = &types.MultiMatchQuery{
				Fields:     fields,
				Query:      value,
				QueryName_: &name,
			}
			if fuzziness != "" {
				query.MultiMatch.Fuzziness = fuzziness
			}
		}
	}
}

// MultiMatchPrefix adds a "MultiMatch" query on the given field with the given value and performing a phrase_prefix
// search.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-multi-match-query#type-phrase
func MultiMatchPrefix(value string, fields ...string) Option {
	return func(query *types.Query) {
		if value != "" {
			name := "multi-match-prefix-" + strings.Join(fields, "+")
			query.MultiMatch = &types.MultiMatchQuery{
				Fields:     fields,
				Query:      value,
				QueryName_: &name,
				Type:       &textquerytype.Phraseprefix,
			}
		}
	}
}

// Distance adds a Distance Feature query.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-distance-feature-query
func Distance(field, pivot, origin string) Option {
	return func(query *types.Query) {
		name := "distance-" + field
		query.DistanceFeature = &types.DateDistanceFeatureQuery{
			Field:      field,
			Pivot:      pivot,
			Origin:     origin,
			QueryName_: &name,
		}
	}
}

// MoreLikeThisQuery represents a "More Like This" query.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-mlt-query
type MoreLikeThisQuery struct {
	*types.MoreLikeThisQuery
}

// NewMoreLikeThisQuery creates a new object for generating a More Like This query. It can be used to build/add options
// for the query.
func NewMoreLikeThisQuery(id string) *MoreLikeThisQuery {
	mlt := &MoreLikeThisQuery{
		MoreLikeThisQuery: types.NewMoreLikeThisQuery(),
	}
	mlt.QueryName_ = &id
	return mlt
}

// LikeDocs adds the given document IDs to the More Like This query for matching.
func (mlt *MoreLikeThisQuery) LikeDocs(ids ...string) {
	likeDocs := make([]types.Like, 0, len(ids))
	for id := range slices.Values(ids) {
		likeDocs = append(likeDocs, types.LikeDocument{Id_: &id})
	}
	mlt.Like = append(mlt.Like, likeDocs...)
}

// ToQueryOption adds the More Like This query to the query object.
func (mlt *MoreLikeThisQuery) ToQueryOption() Option {
	return func(query *types.Query) {
		query.MoreLikeThis = mlt.MoreLikeThisQuery
	}
}

// Term adds a "Term" query on the given field with the given value.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
func Term(field string, value any) Option {
	return func(query *types.Query) {
		name := "term-" + field
		switch v := value.(type) {
		case string:
			if v != "" {
				query.Term = map[string]types.TermQuery{
					field: {Value: value, QueryName_: &name},
				}
			}
		default:
			query.Term = map[string]types.TermQuery{
				field: {Value: value, QueryName_: &name},
			}
		}
	}
}

// Terms adds a "Terms" query on the given field with the given string value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-terms-query
func Terms[T any](field string, values ...T) Option {
	return func(query *types.Query) {
		if len(values) > 0 {
			terms := make([]T, 0, len(values))
			for value := range slices.Values(values) {
				terms = append(terms, value)
			}
			query.Terms = &types.TermsQuery{
				TermsQuery: map[string]types.TermsQueryField{
					field: terms,
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

		name := "since-" + field

		query.Range = map[string]types.RangeQuery{
			field: types.DateRangeQuery{
				Gte:        &sinceStr,
				QueryName_: &name,
			},
		}
	}
}

// Before adds a "Range" query to find documents older than the given time.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func Before(field string, before time.Time) Option {
	return func(query *types.Query) {
		var beforeStr string
		if before.IsZero() {
			return
		}
		beforeStr = before.UTC().Format(time.RFC3339Nano)

		name := "before-" + field

		query.Range = map[string]types.RangeQuery{
			field: types.DateRangeQuery{
				Lte:        &beforeStr,
				QueryName_: &name,
			},
		}
	}
}

// Between adds a "Range" query to find documents between (or equal to) the given times.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func Between(field string, from time.Time, to time.Time) Option {
	return func(query *types.Query) {
		if !from.IsZero() && !to.IsZero() {
			fromStr := from.UTC().Format(time.RFC3339Nano)
			toStr := to.UTC().Format(time.RFC3339Nano)
			name := "between-" + field
			query.Range = map[string]types.RangeQuery{
				field: types.DateRangeQuery{
					Gte:        &fromStr,
					Lte:        &toStr,
					QueryName_: &name,
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
		for option := range slices.Values(options) {
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

// SimpleQueryString constructs a simple query string query and adds it to the query.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-simple-query-string-query
func SimpleQueryString(text *string, flags string, fields ...string) Option {
	return func(query *types.Query) {
		// Don't add the query if the text is the zero value.
		if text == nil || *text == "" {
			return
		}
		name := "simple-query-string-" + strings.Join(fields, "+")
		query.SimpleQueryString = types.NewSimpleQueryStringQuery()
		query.SimpleQueryString.Fields = fields
		query.SimpleQueryString.Query = *text
		query.SimpleQueryString.QueryName_ = &name
		if flags != "" {
			query.SimpleQueryString.Flags = flags
		}
	}
}

// SearchAsYouType constructs a search-as-you-type query for suggesting text as the user types.
//
// https://www.elastic.co/docs/reference/elasticsearch/mapping-reference/search-as-you-type
func SearchAsYouType(text string, field string) Option {
	return func(query *types.Query) {
		name := "search-as-you-type-" + field
		mmq := types.NewMultiMatchQuery()
		mmq.Query = text
		mmq.Type = &textquerytype.Boolprefix
		mmq.Fields = []string{
			field + ".search",
			field + ".search._2gram",
			field + ".search._3gram",
		}
		mmq.QueryName_ = &name
		query.MultiMatch = mmq
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

// BoolOption is a functional option for a boolean query.
type BoolOption func(*types.BoolQuery)

// Filter sets the given query options as the "filter" clause of the bool query.
//
// The clause (query) must appear in matching documents. However unlike must the score of the query will be ignored.
// Filter clauses are executed in filter context, meaning that scoring is ignored and clauses are considered for
// caching. Each query defined under a filter acts as a logical "AND", returning only documents that match all the
// specified queries.
func Filter(queryOptions ...Option) BoolOption {
	return func(boolQueryClause *types.BoolQuery) {
		var filters []types.Query
		// Create queries for each of the passed in query options and append to
		// the bool filters list.
		for _, queryOption := range queryOptions {
			filterClause := &types.Query{}
			if queryOption != nil {
				queryOption(filterClause)
				if !reflect.DeepEqual(filterClause, &types.Query{}) {
					filters = append(filters, *filterClause)
				}
			}
		}

		// Create the filter clause.
		if len(filters) > 0 {
			boolQueryClause.Filter = filters
		}
	}
}

// Must sets the given query options as the "must" clause of the bool query.
//
// The clause (query) must appear in matching documents and will contribute to the score. Each query defined under a
// must acts as a logical "AND", returning only documents that match all the specified queries.
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
//
// The clause (query) must not appear in the matching documents. Clauses are executed in filter context meaning that
// scoring is ignored and clauses are considered for caching. Because scoring is ignored, a score of 0 for all documents
// is returned. Each query defined under a must_not acts as a logical "NOT", returning only documents that do not match
// any of the specified queries.
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
//
// The clause (query) should appear in the matching document. Each query defined under a should acts as a logical "OR",
// returning documents that match any of the specified queries.
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

// WithBoolQueryName option assigns the given string as the name of the query, which allows
// tracking when this bool clause matches documents.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-bool-query.html#named-queries
func WithBoolQueryName(name string) BoolOption {
	return func(boolQueryClause *types.BoolQuery) {
		if name != "" {
			boolQueryClause.QueryName_ = &name
		}
	}
}

// WithBoolQueryBoost option adds the given boost to the bool query.
// tracking when this bool clause matches documents.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-bool-query.html#named-queries
func WithBoolQueryBoost(boost float32) BoolOption {
	return func(boolQueryClause *types.BoolQuery) {
		boolQueryClause.Boost = &boost
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

// func (mso *MsearchSearch) GenerateSortCombination() []types.SortCombinations {
// 	combos := make([]types.SortCombinations, 0, len(mso.Sort))
// 	for sort := range slices.Values(mso.Sort) {
// 		combos = append(combos, sort.SortCombinationsCaster())
// 	}
// 	return combos
// }
