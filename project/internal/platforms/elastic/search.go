// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"errors"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
)

var ErrSearchFailed = errors.New("search failed")

type QueryOption func(*types.Query) *types.Query

// QueryMatchAll adds a "Match All" clause.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
func QueryMatchAll() QueryOption {
	return func(q *types.Query) *types.Query {
		q.MatchAll = types.NewMatchAllQuery()
		return q
	}
}

func QueryByTerm(field string, value any) QueryOption {
	return func(q *types.Query) *types.Query {
		q.Term = map[string]types.TermQuery{
			field: {Value: value},
		}

		return q
	}
}

// QueryByFeedIDs adds a "Terms" clause with the given Feed IDs.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-terms-query.html
func QueryByFeedIDs(feedIDs ...string) QueryOption {
	return func(q *types.Query) *types.Query {
		q.Terms = &types.TermsQuery{
			TermsQuery: map[string]types.TermsQueryField{
				"feed_id": feedIDs,
			},
		}

		return q
	}
}

// QuerySince adds a "Range" query to find documents newer than the given time.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func QuerySince(since time.Time) QueryOption {
	return func(q *types.Query) *types.Query {
		sinceStr := since.String()
		q.Range = map[string]types.RangeQuery{
			"@timestamp": types.DateRangeQuery{
				Gte: &sinceStr,
			},
		}

		return q
	}
}

func QueryOptions(options ...QueryOption) *types.Query {
	queryOptions := &types.Query{}

	for _, option := range options {
		queryOptions = option(queryOptions)
	}

	return queryOptions
}

type SearchOption func(*search.Search) *search.Search

// IndexPattern defines the index pattern the search will use.
func IndexPattern(pattern string) SearchOption {
	return func(s *search.Search) *search.Search {
		s = s.Index(pattern)
		return s
	}
}

// WithQueryOptions adds the given query options (conditions) to the search.
func WithQueryOptions(options ...QueryOption) SearchOption {
	return func(s *search.Search) *search.Search {
		queryOptions := &types.Query{}

		for _, option := range options {
			queryOptions = option(queryOptions)
		}

		s.Query(queryOptions)

		return s
	}
}

// WithSortOptions adds the given sorting options to the search.
func WithSortOptions(options map[string]types.FieldSort) SearchOption {
	return func(s *search.Search) *search.Search {
		s = s.Sort(options)
		return s
	}
}

// WithFields ensures the search will return the given fields in the response.
func WithFields(fields ...string) SearchOption {
	return func(s *search.Search) *search.Search {
		fieldsReturned := make([]types.FieldAndFormat, len(fields))
		for i, name := range fields {
			fieldsReturned[i] = types.FieldAndFormat{Field: name}
		}

		s.Fields(fieldsReturned...)

		return s
	}
}

// SearchSize defines the number of results returned.
func SearchSize(size int) SearchOption {
	return func(s *search.Search) *search.Search {
		s = s.Size(size)
		return s
	}
}

func (c *Client) NewSearchRequest(options ...SearchOption) *search.Search {
	req := c.API.Search()

	for _, option := range options {
		req = option(req)
	}

	return req
}

func SortTimestampDesc() map[string]types.FieldSort {
	return map[string]types.FieldSort{
		"@timestamp": {
			Order: &sortorder.Desc,
		},
	}
}
