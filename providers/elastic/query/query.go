// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package query contains methods for building Elasticsearch queries.
package query

import (
	"reflect"
	"slices"
	"time"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/operator"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/textquerytype"
)

// Option is a functional option for queries.
type Option func(*types.Query)

// NumberRangeOption is a functional option for a number range query.
type NumberRangeOption func(*types.NumberRangeQuery)

type MatchAllQuery struct {
	*types.MatchAllQuery
}

func (q *MatchAllQuery) SetName(name string) {
	q.QueryName_ = &name
}

func (q *MatchAllQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

// MatchAll adds a "Match All" clause.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
func MatchAll(options ...func(*MatchAllQuery)) Option {
	return func(query *types.Query) {
		q := &MatchAllQuery{
			MatchAllQuery: types.NewMatchAllQuery(),
		}
		for option := range slices.Values(options) {
			option(q)
		}
		query.MatchAll = q.MatchAllQuery
	}
}

type ExistsQuery struct {
	*types.ExistsQuery
}

func (q *ExistsQuery) SetName(name string) {
	q.QueryName_ = &name
}

func (q *ExistsQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

// Exists query adds an "Exists" clause.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-exists-query
func Exists(field string, options ...func(*ExistsQuery)) Option {
	return func(query *types.Query) {
		q := &ExistsQuery{
			&types.ExistsQuery{
				Field: field,
			},
		}
		for option := range slices.Values(options) {
			option(q)
		}
		query.Exists = q.ExistsQuery
	}
}

type MatchQuery struct {
	*types.MatchQuery
}

func (q *MatchQuery) SetName(name string) {
	q.QueryName_ = &name
}

func (q *MatchQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

func (q *MatchQuery) SetFuzziness(fuzziness FuzzinessValue) {
	q.Fuzziness = fuzziness
}

func (q *MatchQuery) SetFuzzyTranspositions(value bool) {
	q.FuzzyTranspositions = &value
}

// Match adds a "Match" query on the given field with the given value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-match-query
func Match(field, value string, options ...func(*MatchQuery)) Option {
	return func(query *types.Query) {
		if value != "" {
			q := &MatchQuery{
				&types.MatchQuery{
					Query: value,
				},
			}
			for option := range slices.Values(options) {
				option(q)
			}
			if query.Match == nil {
				query.Match = make(map[string]types.MatchQuery)
			}
			query.Match[field] = *q.MatchQuery
		}
	}
}

type MatchPhraseQuery struct {
	*types.MatchPhraseQuery
}

func (q *MatchPhraseQuery) SetName(name string) {
	q.QueryName_ = &name
}

func (q *MatchPhraseQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

func (q *MatchPhraseQuery) SetSlop(slop int) {
	q.Slop = &slop
}

// MatchPhrase adds a "MatchPhrase" query on the given field with the given value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-match-query-phrase
func MatchPhrase(field, value string, options ...func(*MatchPhraseQuery)) Option {
	return func(query *types.Query) {
		if value != "" {
			q := &MatchPhraseQuery{
				&types.MatchPhraseQuery{
					Query: value,
				},
			}
			for option := range slices.Values(options) {
				option(q)
			}
			if query.MatchPhrase == nil {
				query.MatchPhrase = make(map[string]types.MatchPhraseQuery)
			}
			query.MatchPhrase[field] = *q.MatchPhraseQuery
		}
	}
}

type MultiMatchQuery struct {
	*types.MultiMatchQuery
}

func (q *MultiMatchQuery) SetName(name string) {
	q.QueryName_ = &name
}

func (q *MultiMatchQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

func (q *MultiMatchQuery) SetFuzziness(fuzziness FuzzinessValue) {
	q.Fuzziness = fuzziness
}

func (q *MultiMatchQuery) SetFuzzyTranspositions(value bool) {
	q.FuzzyTranspositions = &value
}

func (q *MultiMatchQuery) SetSlop(slop int) {
	q.Slop = &slop
}

func (q *MultiMatchQuery) SetTextQueryType(tq TextQueryType) {
	q.Type = new(textquerytype.TextQueryType(tq))
}

// MultiMatch adds a "MultiMatch" query on the given field with the given value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-multi-match-query
func MultiMatch(value string, fields []string, options ...func(*MultiMatchQuery)) Option {
	return func(query *types.Query) {
		if value != "" {
			q := &MultiMatchQuery{
				&types.MultiMatchQuery{
					Fields: fields,
					Query:  value,
				},
			}
			for option := range slices.Values(options) {
				option(q)
			}
			query.MultiMatch = q.MultiMatchQuery
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

type TermQuery struct {
	*types.TermQuery
}

func (q *TermQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

func (q *TermQuery) SetName(name string) {
	q.QueryName_ = &name
}

// Term adds a "Term" query on the given field with the given value.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
func Term(field string, value any, options ...func(*TermQuery)) Option {
	return func(query *types.Query) {
		// Create term query clause.
		termQueryClause := &TermQuery{
			&types.TermQuery{
				Value: value,
			},
		}
		// Set options.
		for option := range slices.Values(options) {
			option(termQueryClause)
		}
		// Apply term query clause to query.
		if query.Term == nil {
			query.Term = make(map[string]types.TermQuery)
		}
		query.Term[field] = *termQueryClause.TermQuery
	}
}

type TermsQuery struct {
	*types.TermsQuery
}

func (q *TermsQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

func (q *TermsQuery) SetName(name string) {
	q.QueryName_ = &name
}

// Terms adds a "Terms" query on the given field with the given string value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-terms-query
func Terms[T any](field string, values []T, options ...func(*TermsQuery)) Option {
	return func(query *types.Query) {
		if len(values) > 0 {
			terms := make([]T, 0, len(values))
			for value := range slices.Values(values) {
				terms = append(terms, value)
			}
			// Create terms query clause.
			termsQueryClause := &TermsQuery{
				&types.TermsQuery{
					TermsQuery: map[string]types.TermsQueryField{
						field: terms,
					},
				},
			}
			// Apply options.
			for option := range slices.Values(options) {
				option(termsQueryClause)
			}
			// Apply terms query clause to query.
			query.Terms = termsQueryClause.TermsQuery
		}
	}
}

type WildcardQuery struct {
	*types.WildcardQuery
}

func (q *WildcardQuery) SetBoost(boost float32) {
	q.Boost = &boost
}

func (q *WildcardQuery) SetName(name string) {
	q.QueryName_ = &name
}

// Wildcard adds a "Wildcard" query on the given field with the given value.
//
// https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-wildcard-query
func Wildcard(field string, value string, options ...func(*WildcardQuery)) Option {
	return func(query *types.Query) {
		// Create term query clause.
		wildcardQueryClause := &WildcardQuery{
			&types.WildcardQuery{
				Value: &value,
			},
		}
		// Set options.
		for option := range slices.Values(options) {
			option(wildcardQueryClause)
		}
		// Apply term query clause to query.
		if query.Wildcard == nil {
			query.Wildcard = make(map[string]types.WildcardQuery)
		}
		query.Wildcard[field] = *wildcardQueryClause.WildcardQuery
	}
}

// Since adds a "Range" query to find documents newer than the given time.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func Since(field string, since time.Time) Option {
	return func(query *types.Query) {
		var sinceStr string
		if since.IsZero() {
			return
		}
		sinceStr = since.UTC().Format(time.RFC3339Nano)
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
func SimpleQueryString(options ...SimpleQueryStringOption) Option {
	return func(query *types.Query) {
		simpleQueryString := types.NewSimpleQueryStringQuery()
		for option := range slices.Values(options) {
			option(simpleQueryString)
		}

		// Only apply the clause if it is not the zero value and has valid query/fields set.
		if !reflect.DeepEqual(simpleQueryString, &types.SimpleQueryStringQuery{}) && simpleQueryString.Query != "" &&
			len(simpleQueryString.Fields) > 0 {
			query.SimpleQueryString = simpleQueryString
		}
	}
}

// SimpleQueryStringOption is a functional option for building a SimpleQueryString query.
type SimpleQueryStringOption func(*types.SimpleQueryStringQuery)

// WithSimpleQueryStringName option assigns a name to the query.
func WithSimpleQueryStringName(name string) SimpleQueryStringOption {
	return func(q *types.SimpleQueryStringQuery) {
		q.QueryName_ = &name
	}
}

// WithSimpleQueryStringText option specifies the query text.
func WithSimpleQueryStringText(text *string) SimpleQueryStringOption {
	return func(q *types.SimpleQueryStringQuery) {
		if text != nil && *text != "" {
			q.Query = *text
		}
	}
}

// WithSimpleQueryStringFields option specifies the fields to query.
func WithSimpleQueryStringFields(fields ...string) SimpleQueryStringOption {
	return func(q *types.SimpleQueryStringQuery) {
		q.Fields = slices.Compact(slices.Concat(q.Fields, fields))
	}
}

// WithSimpleQueryStringOperator option sets the default operator for the SimpleQueryString query. If not used, the
// default is OR.
func WithSimpleQueryStringOperator(op *operator.Operator) SimpleQueryStringOption {
	return func(q *types.SimpleQueryStringQuery) {
		if op != nil {
			q.DefaultOperator = op
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

			if queryOption != nil {
				filterClause := &types.Query{}
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
