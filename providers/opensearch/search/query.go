// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package search

import (
	"slices"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

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
	}
}

// TermsQuery matches documents with a field with the given value.
// NOTE: using a bool until its clearer how to constuct a terms query with the typed API.
//
// https://docs.opensearch.org/latest/query-dsl/term/terms/
func TermsQuery[V any](field string, values []V, options ...CommonQueryOption) QueryOption {
	clauses := make([]QueryOption, 0, len(values))
	for v := range slices.Values(values) {
		clauses = append(clauses, TermQuery(field, v, options...))
	}
	return BoolQuery(
		WithBoolMinimumShouldMatch(1),
		WithBoolShould(clauses...),
	)
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

// BoolQuery constructs a boolean query from the given option to match documents.
//
// https://docs.opensearch.org/latest/query-dsl/compound/bool/
func BoolQuery(options ...BoolQueryOption) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		boolQuery := &opensearchapi.CommonQueryDSLBoolQuery{}
		for option := range slices.Values(options) {
			option(boolQuery)
		}
		q.Bool = boolQuery
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
		filters := make([]opensearchapi.CommonQueryDSLQueryContainer, 0, len(queries))
		for query := range slices.Values(queries) {
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			filters = append(filters, container)
		}
		q.Filter = new(opensearchapi.NewCommonQueryDSLBoolQueryFilterFromArray(filters))
	}
}

// WithBoolMust option defines queries that will be placed within a bool must clause.
func WithBoolMust(queries ...QueryOption) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		musts := make([]opensearchapi.CommonQueryDSLQueryContainer, 0, len(queries))
		for query := range slices.Values(queries) {
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			musts = append(musts, container)
		}
		q.Must = new(opensearchapi.NewCommonQueryDSLBoolQueryMustFromArray(musts))
	}
}

// WithBoolMustNot option defines queries that will be placed within a bool must_not clause.
func WithBoolMustNot(queries ...QueryOption) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		mustNots := make([]opensearchapi.CommonQueryDSLQueryContainer, 0, len(queries))
		for query := range slices.Values(queries) {
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			mustNots = append(mustNots, container)
		}
		q.MustNot = new(opensearchapi.NewCommonQueryDSLBoolQueryMustNotFromArray(mustNots))
	}
}

// WithBoolShould option defines queries that will be placed within a bool should clause.
func WithBoolShould(queries ...QueryOption) BoolQueryOption {
	return func(q *opensearchapi.CommonQueryDSLBoolQuery) {
		shoulds := make([]opensearchapi.CommonQueryDSLQueryContainer, 0, len(queries))
		for query := range slices.Values(queries) {
			container := opensearchapi.CommonQueryDSLQueryContainer{}
			query(&container)
			shoulds = append(shoulds, container)
		}
		q.Should = new(opensearchapi.NewCommonQueryDSLBoolQueryShouldFromArray(shoulds))
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
