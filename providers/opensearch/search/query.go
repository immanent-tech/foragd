// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package search

import (
	"slices"
	"time"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

type QueryOption func(*opensearchapi.CommonQueryDSLQueryContainer)

func MatchAllQuery() QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		q.MatchAll = &opensearchapi.CommonQueryDSLQueryBase{}
	}
}

func TermQuery(field string, value any) QueryOption {
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
		q.Term[field] = opensearchapi.NewCommonQueryDSLTermQueryFromValue(opensearchapi.CommonQueryDSLTermQueryValue{
			Value: &fieldValue,
		})
	}
}

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

type DateRange struct {
	Lower        any
	IncludeLower bool
	Upper        any
	IncludeUpper bool
}

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

func SinceQuery(field string, since any) QueryOption {
	return func(q *opensearchapi.CommonQueryDSLQueryContainer) {
		DateRangeQuery(field, DateRange{
			Lower:        since,
			IncludeLower: true,
		})(q)
	}
}
