// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package search

import (
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

const (
	OperatorAnd Operator = "and"
	OperatorOr  Operator = "or"
)

type Operator string

func (o Operator) String() string {
	return string(o)
}

// QueryOption is a functional option to configure a query for a search request.
type QueryOption func(*opensearchapi.CommonQueryDSLQueryContainer)

// MatchAllQuery matches all documents.
//
// https://docs.opensearch.org/latest/query-dsl/match-all/
func MatchAllQuery() QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		q.MatchAll = &opensearchapi.CommonQueryDSLQueryBase{}
	}
}

// TermQuery matches documents with a field with the given value.
//
// https://docs.opensearch.org/latest/query-dsl/term/term/
func TermQuery(field string, value any, options ...CommonQueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		if value == nil {
			return
		}
		var fieldValue opensearchapi.FieldValue
		switch v := value.(type) {
		case bool:
			fieldValue = opensearchapi.NewFieldValueFromBool(v)
		case float64:
			fieldValue = opensearchapi.NewFieldValueFromFloat64(v)
		case string:
			// Ignore empty string values.
			if v == "" {
				return
			}
			fieldValue = opensearchapi.NewFieldValueFromString(v)
		default:
			return
		}
		if q.Term == nil {
			q.Term = make(map[string]opensearchapi.CommonQueryDSLTermQuery)
		}
		value := opensearchapi.CommonQueryDSLTermQueryValue{
			Value: &fieldValue,
		}
		for option := range slices.Values(options) {
			option(&value.CommonQueryDSLQueryBase)
		}
		q.Term[field] = opensearchapi.NewCommonQueryDSLTermQueryFromValue(value)
		q.Terms = &opensearchapi.CommonQueryDSLTermsQuery{}
	}
}

// TermsQuery matches documents with a field with the given value.
// NOTE: using a bool until its clearer how to constuct a terms query with the typed API.
//
// https://docs.opensearch.org/latest/query-dsl/term/terms/
func TermsQuery[V any](field string, values []V, options ...CommonQueryOption) QueryOption {
	if len(values) == 0 {
		return nil
	}
	clauses := make([]QueryOption, 0, len(values))
	for v := range slices.Values(values) {
		q := TermQuery(field, v, options...)
		if q != nil {
			clauses = append(clauses, q)
		}
	}
	return BoolQuery(
		WithBoolShould(clauses...),
	)
}

// TermsQueryOption is a functional option applied to a [TermsQuery].
type TermsQueryOption func(*opensearchapi.CommonQueryDSLTermsQuery)

// func TermsQueryFromQuery(field, index, path string, query QueryOption) TermsQueryOption {
// 	return func(q *opensearchapi.CommonQueryDSLTermsQuery) {
// 		var termsQuery map[string]struct {
// 			Index string `json:"index"`
// 			Path  string `json:"path"`
// 			Query json.RawMessage
// 		}

// 	}
// }

// MatchQuery performs full-text matching on a field with a value.
//
// https://docs.opensearch.org/latest/query-dsl/full-text/match/
func MatchQuery(field string, value any, options ...CommonQueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		if value == nil {
			return
		}
		var fieldValue opensearchapi.FieldValue
		switch v := value.(type) {
		case bool:
			fieldValue = opensearchapi.NewFieldValueFromBool(v)
		case float64:
			fieldValue = opensearchapi.NewFieldValueFromFloat64(v)
		case string:
			fieldValue = opensearchapi.NewFieldValueFromString(v)
		default:
			return
		}
		if q.Match == nil {
			q.Match = make(map[string]opensearchapi.CommonQueryDSLMatchQuery)
		}
		value := opensearchapi.CommonQueryDSLMatchQueryQuery{
			Query: &fieldValue,
		}
		for option := range slices.Values(options) {
			option(&value.CommonQueryDSLQueryBase)
		}
		q.Match[field] = opensearchapi.NewCommonQueryDSLMatchQueryFromQuery(value)
	}
}

// MultiMatchQuery runs a match query against multiple fields at once.
//
// https://docs.opensearch.org/latest/query-dsl/full-text/multi-match/
func MultiMatchQuery(value string, fields []string, options ...MultiMatchOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		mm := &opensearchapi.CommonQueryDSLMultiMatchQuery{
			Query:  value,
			Fields: new(strings.Join(fields, ",")),
		}
		for option := range slices.Values(options) {
			option(mm)
		}
		q.MultiMatch = mm
	}
}

type MultiMatchOption func(*opensearchapi.CommonQueryDSLMultiMatchQuery)

func WithMultiMatchFuzziness(fuzz string) MultiMatchOption {
	return func(m *opensearchapi.CommonQueryDSLMultiMatchQuery) {
		m.Fuzziness = &fuzz
	}
}

func WithMultiMatchSlop(slop int) MultiMatchOption {
	return func(m *opensearchapi.CommonQueryDSLMultiMatchQuery) {
		m.Slop = &slop
	}
}

func WithMultiMatchType(typ string) MultiMatchOption {
	return func(m *opensearchapi.CommonQueryDSLMultiMatchQuery) {
		m.Type = &typ
	}
}

// SearchAsYouTypeQuery uses a [MultiMatchQuery] to perform search-as-you-type suggestion matches on a field configured with
// a "search_as_you_type" mapping.
//
// https://docs.opensearch.org/latest/mappings/supported-field-types/search-as-you-type/
func SearchAsYouTypeQuery(value, field string) QueryOption {
	return MultiMatchQuery(
		value,
		[]string{
			field + ".search",
			field + ".search._2gram",
			field + ".search._3gram",
		},
		WithMultiMatchType("bool_prefix"),
	)
}

func SimpleQueryStringQuery(query string, fields []string, options ...SimpleQueryStringQueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		if query == "" {
			return
		}
		qs := &opensearchapi.CommonQueryDSLSimpleQueryStringQuery{
			Query:  query,
			Fields: fields,
		}
		for option := range slices.Values(options) {
			option(qs)
		}
		q.SimpleQueryString = qs
	}
}

type SimpleQueryStringQueryOption func(*opensearchapi.CommonQueryDSLSimpleQueryStringQuery)

func WithSimpleQueryStringQueryOperator(op Operator) SimpleQueryStringQueryOption {
	return func(q *opensearchapi.CommonQueryDSLSimpleQueryStringQuery) {
		q.DefaultOperator = new(op.String())
	}
}

// ExistsQuery matches documents that have the given field.
//
// https://docs.opensearch.org/latest/query-dsl/term/exists/
func ExistsQuery(field string, options ...CommonQueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		q.Exists = &opensearchapi.CommonQueryDSLExistsQuery{
			Field: field,
		}
		for option := range slices.Values(options) {
			option(&q.Exists.CommonQueryDSLQueryBase)
		}
	}
}

// DateRangeQuery matches documents with a date field within the given range.
//
// https://docs.opensearch.org/latest/query-dsl/term/range/
func DateRangeQuery(field string, value DateRange, options ...DateRangeOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		drq := &opensearchapi.CommonQueryDSLDateRangeQuery{}
		// Set range bounds.
		if value.Lower != nil {
			var lowerStr string
			switch v := value.Lower.(type) {
			case time.Time:
				lowerStr = v.UTC().Format(time.RFC3339Nano)
			case string:
				lowerStr = v
			}
			if value.IncludeLower {
				drq.Gte = &lowerStr
			} else {
				drq.Gt = &lowerStr
			}
		}
		if value.Upper != nil {
			var upperStr string
			switch v := value.Upper.(type) {
			case time.Time:
				upperStr = v.UTC().Format(time.RFC3339Nano)
			case string:
				upperStr = v
			}
			if value.IncludeUpper {
				drq.Lte = &upperStr
			} else {
				drq.Lt = &upperStr
			}
		}

		// Add options.
		for option := range slices.Values(options) {
			option(drq)
		}
		if q.Range == nil {
			q.Range = make(map[string]opensearchapi.CommonQueryDSLRangeQuery)
		}
		q.Range[field] = opensearchapi.NewCommonQueryDSLRangeQueryFromDateRangeQuery(*drq)
	}
}

// DateRange contains the parameters for defining a [DateRangeQuery].
type DateRange struct {
	Lower        any
	IncludeLower bool
	Upper        any
	IncludeUpper bool
}

// DateRangeOption is a functional option to configure a [DateRangeQuery].
type DateRangeOption func(*opensearchapi.CommonQueryDSLDateRangeQuery)

// WithDateFormat option specifies a date format other than the field’s mapped format to use in the query.
func WithDateFormat(format string) DateRangeOption {
	return func(q *opensearchapi.CommonQueryDSLDateRangeQuery) {
		q.Format = &format
	}
}

// WithTimezone option specifies the timezone of values used in the query. They will be converted to UTC when querying
// the field.
func WithTimezone(tz string) DateRangeOption {
	return func(q *opensearchapi.CommonQueryDSLDateRangeQuery) {
		q.TimeZone = &tz
	}
}

// SinceQuery matches documents with a datetime field that has a value equal to or greater than the given value. This is
// a wrapper around [DateRangeQuery].
func SinceQuery(field string, since any) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		DateRangeQuery(field, DateRange{
			Lower:        since,
			IncludeLower: true,
		})(q)
	}
}

// BeforeQuery matches documents with a datetime field that has a value equal to or less than the given value.  This is
// a wrapper around [DateRangeQuery].
func BeforeQuery(field string, before any) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		DateRangeQuery(field, DateRange{
			Upper:        before,
			IncludeUpper: true,
		})(q)
	}
}

// DistanceFeatureDateQuery boosts the relevance of documents that are closer to a specific date.
//
// https://docs.opensearch.org/latest/query-dsl/specialized/distance-feature/
func DistanceFeatureDateQuery(field, origin, pivot string) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		q.DistanceFeature = new(opensearchapi.NewCommonQueryDSLDistanceFeatureQueryFromObject1(
			opensearchapi.CommonQueryDSLDistanceFeatureQueryObject1{
				Field:  field,
				Origin: origin,
				Pivot:  pivot,
			},
		))
	}
}

// BoolQuery constructs a boolean query from the given option to match documents.
//
// https://docs.opensearch.org/latest/query-dsl/compound/bool/
func BoolQuery(options ...BoolQueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		if len(options) == 0 {
			return
		}
		boolQuery := &opensearchapi.CommonQueryDSLBoolQuery{}
		for option := range slices.Values(options) {
			if option == nil {
				continue
			}
			option(boolQuery)
		}
		if !reflect.DeepEqual(boolQuery, &opensearchapi.CommonQueryDSLBoolQuery{}) {
			q.Bool = boolQuery
		}
	}
}

// BoolQueryOption is a functional option to construct a [BoolQuery].
type BoolQueryOption func(*opensearchapi.CommonQueryDSLBoolQuery)

// WithBoolMinimumShouldMatch option sets the minimum number of Should clauses that must match.
func WithBoolMinimumShouldMatch(min int) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		q.MinimumShouldMatch = new(opensearchapi.NewMinimumShouldMatchFromInt(min))
	}
}

// WithBoolFilter option defines queries that will be placed within a bool filter clause.
func WithBoolFilter(queries ...QueryOption) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		if len(queries) == 0 {
			return
		}
		filters := make([]opensearchapi.CommonQueryDSLQueryContainer, 0)
		for query := range slices.Values(queries) {
			if query == nil {
				continue
			}
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			if !reflect.DeepEqual(container, opensearchapi.CommonQueryDSLQueryContainer{}) {
				filters = append(filters, container)
			}
		}
		clause := opensearchapi.NewCommonQueryDSLBoolQueryFilterFromArray(filters)
		if !reflect.DeepEqual(&clause, &opensearchapi.CommonQueryDSLBoolQueryFilter{}) {
			q.Filter = &clause
		}
	}
}

// WithBoolMust option defines queries that will be placed within a bool must clause.
func WithBoolMust(queries ...QueryOption) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		if len(queries) == 0 {
			return
		}
		musts := make([]opensearchapi.CommonQueryDSLQueryContainer, 0)
		for query := range slices.Values(queries) {
			if query == nil {
				continue
			}
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			if !reflect.DeepEqual(container, opensearchapi.CommonQueryDSLQueryContainer{}) {
				musts = append(musts, container)
			}
		}
		clause := opensearchapi.NewCommonQueryDSLBoolQueryMustFromArray(musts)
		if !reflect.DeepEqual(&clause, &opensearchapi.CommonQueryDSLBoolQueryMust{}) {
			q.Must = &clause
		}
	}
}

// WithBoolMustNot option defines queries that will be placed within a bool must_not clause.
func WithBoolMustNot(queries ...QueryOption) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		if len(queries) == 0 {
			return
		}
		mustNots := make([]opensearchapi.CommonQueryDSLQueryContainer, 0)
		for query := range slices.Values(queries) {
			if query == nil {
				continue
			}
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			if !reflect.DeepEqual(container, opensearchapi.CommonQueryDSLQueryContainer{}) {
				mustNots = append(mustNots, container)
			}
		}
		clause := opensearchapi.NewCommonQueryDSLBoolQueryMustNotFromArray(mustNots)
		if !reflect.DeepEqual(&clause, &opensearchapi.CommonQueryDSLBoolQueryMustNot{}) {
			q.MustNot = &clause
		}
	}
}

// WithBoolShould option defines queries that will be placed within a bool should clause.
func WithBoolShould(queries ...QueryOption) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		if len(queries) == 0 {
			return
		}
		shoulds := make([]opensearchapi.CommonQueryDSLQueryContainer, 0)
		for query := range slices.Values(queries) {
			if query == nil {
				continue
			}
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			if !reflect.DeepEqual(container, opensearchapi.CommonQueryDSLQueryContainer{}) {
				shoulds = append(shoulds, container)
			}
		}
		clause := opensearchapi.NewCommonQueryDSLBoolQueryShouldFromArray(shoulds)
		if !reflect.DeepEqual(&clause, &opensearchapi.CommonQueryDSLBoolQueryShould{}) {
			q.Should = &clause
		}
	}
}

type CommonQueryOption func(*opensearchapi.CommonQueryDSLQueryBase)

// WithQueryName option defines a friendly name for the query that can be referenced in the response.
func WithQueryName(name string) CommonQueryOption {
	return func(b *opensearchapi.CommonQueryDSLQueryBase) {
		b.Name = &name
	}
}

// WithQueryBoost option defines a boost value to this query to increase or decrease its score relative to other
// matches.
func WithQueryBoost(boost float32) CommonQueryOption {
	return func(b *opensearchapi.CommonQueryDSLQueryBase) {
		b.Boost = &boost
	}
}

// HybridQuery builds a hybrid query from the filter and queries.
//
// https://docs.opensearch.org/latest/vector-search/ai-search/hybrid-search/index/
func HybridQuery(filters QueryOption, queries ...QueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		hybrid := &opensearchapi.CommonQueryDSLHybridQuery{
			Queries: make([]opensearchapi.CommonQueryDSLQueryContainer, 0),
		}
		if filters != nil {
			hybrid.Filter = &opensearchapi.CommonQueryDSLQueryContainer{}
			filters(hybrid.Filter)
		}
		for queryOption := range slices.Values(queries) {
			q := &opensearchapi.CommonQueryDSLQueryContainer{}
			queryOption(q)
			if !reflect.DeepEqual(q, &opensearchapi.CommonQueryDSLQueryContainer{}) {
				hybrid.Queries = append(hybrid.Queries, *q)
			}
		}
		q.Hybrid = hybrid
	}
}

// MoreLikeThisQuery builds a more_like_this query against the given fields using the given options.
//
// https://docs.opensearch.org/latest/query-dsl/specialized/more-like-this/
func MoreLikeThisQuery(fields []string, options ...MoreLikeThisQueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		mlt := &opensearchapi.CommonQueryDSLMoreLikeThisQuery{
			Fields: fields,
		}
		for option := range slices.Values(options) {
			option(mlt)
		}
		q.MoreLikeThis = mlt
	}
}

// MoreLikeThisQueryOption is a functional option to apply to a [MoreLikeThisQuery].
type MoreLikeThisQueryOption func(*opensearchapi.CommonQueryDSLMoreLikeThisQuery)

// WithMoreLikeThisQueryLikeDocIDs option sets the existing document IDs for which to find similar documents.
func WithMoreLikeThisQueryLikeDocIDs(ids ...string) MoreLikeThisQueryOption {
	return func(mlt *opensearchapi.CommonQueryDSLMoreLikeThisQuery) {
		likes := make([]opensearchapi.CommonQueryDSLLike, 0, len(ids))
		for id := range slices.Values(ids) {
			like := opensearchapi.NewCommonQueryDSLLikeFromLikeDocument(
				opensearchapi.CommonQueryDSLLikeDocument{ID: &id},
			)
			likes = append(likes, like)
		}
		mlt.Like = opensearchapi.NewCommonQueryDSLMoreLikeThisQueryLikeFromArray(likes)
	}
}

// WithMoreLikeThisQueryMinTermFreq option sets the minimum term frequency below which the terms are ignored from the
// input document.
func WithMoreLikeThisQueryMinTermFreq(freq int) MoreLikeThisQueryOption {
	return func(mlt *opensearchapi.CommonQueryDSLMoreLikeThisQuery) {
		mlt.MinTermFreq = &freq
	}
}

// WithMoreLikeThisQueryMinDocFreq option sets the minimum document frequency below which the terms are ignored from the
// input document.
func WithMoreLikeThisQueryMinDocFreq(freq int) MoreLikeThisQueryOption {
	return func(mlt *opensearchapi.CommonQueryDSLMoreLikeThisQuery) {
		mlt.MinDocFreq = &freq
	}
}

// WithMoreLikeThisQueryMinWordLen option sets the minimum word length below which the terms are ignored.
func WithMoreLikeThisQueryMinWordLen(length int) MoreLikeThisQueryOption {
	return func(mlt *opensearchapi.CommonQueryDSLMoreLikeThisQuery) {
		mlt.MinWordLength = &length
	}
}

// WithMoreLikeThisQueryMaxQueryTerms option sets the maximum number of query terms that can be selected.
func WithMoreLikeThisQueryMaxQueryTerms(terms int) MoreLikeThisQueryOption {
	return func(mlt *opensearchapi.CommonQueryDSLMoreLikeThisQuery) {
		mlt.MaxQueryTerms = &terms
	}
}
