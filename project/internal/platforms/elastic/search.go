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

var (
	ErrSearchFailed = errors.New("search failed")
	ErrNoHits       = errors.New("no hits found")
)

// QueryMatchAll adds a "Match All" clause.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-match-all-query.html
func QueryMatchAll() Option[*types.Query] {
	return func(query *types.Query) *types.Query {
		query.MatchAll = types.NewMatchAllQuery()
		return query
	}
}

// QueryByTerm adds a "Term" query on the given field with the given value.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-term-query.html
func QueryByTerm(field string, value any) Option[*types.Query] {
	return func(query *types.Query) *types.Query {
		query.Term = map[string]types.TermQuery{
			field: {Value: value},
		}

		return query
	}
}

// QueryByFeedIDs adds a "Terms" clause with the given Feed IDs.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-terms-query.html
func QueryByFeedIDs(feedIDs ...string) Option[*types.Query] {
	return func(query *types.Query) *types.Query {
		query.Terms = &types.TermsQuery{
			TermsQuery: map[string]types.TermsQueryField{
				"feed_id": feedIDs,
			},
		}

		return query
	}
}

// QueryByItemIDs adds a "Terms" clause with the given Item IDs.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-terms-query.html
func QueryByItemIDs(itemIDs ...string) Option[*types.Query] {
	return func(query *types.Query) *types.Query {
		query.Terms = &types.TermsQuery{
			TermsQuery: map[string]types.TermsQueryField{
				"item_id": itemIDs,
			},
		}

		return query
	}
}

// QuerySince adds a "Range" query to find documents newer than the given time.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-range-query.html#ranges-on-dates
func QuerySince(field string, since time.Time) Option[*types.Query] {
	return func(query *types.Query) *types.Query {
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

		return query
	}
}

// WithQueryOptions adds the given query options (conditions) to the search.
func WithQueryOptions(options ...Option[*types.Query]) Option[*search.Search] {
	return func(search *search.Search) *search.Search {
		queryOptions := &types.Query{}

		for _, option := range options {
			queryOptions = option(queryOptions)
		}

		search.Query(queryOptions)

		return search
	}
}

// WithSortOptions adds the given sorting options to the search.
func WithSortOptions(options map[string]types.FieldSort) Option[*search.Search] {
	return func(search *search.Search) *search.Search {
		search = search.Sort(options)
		return search
	}
}

// WithFields ensures the search will return the given fields in the response.
func WithFields(fields ...string) Option[*search.Search] {
	return func(search *search.Search) *search.Search {
		fieldsReturned := make([]types.FieldAndFormat, len(fields))
		for i, name := range fields {
			fieldsReturned[i] = types.FieldAndFormat{Field: name}
		}

		search.Fields(fieldsReturned...)

		return search
	}
}

// SearchSize defines the number of results returned.
func SearchSize(size int) Option[*search.Search] {
	return func(search *search.Search) *search.Search {
		search = search.Size(size)
		return search
	}
}

// SearchAfter sets the sort value to fetch the next set of results.
// https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html#search-after
func SearchAfter(sortValue []types.FieldValue) Option[*search.Search] {
	return func(search *search.Search) *search.Search {
		search = search.SearchAfter(sortValue...)
		return search
	}
}

// NewSearchRequest creates a new search object with the given options.
func (c *Client) NewSearchRequest(options ...Option[*search.Search]) *search.Search {
	req := c.API.Search()

	for _, option := range options {
		req = option(req)
	}

	return req
}

// SortTimestampDesc returns a sort parameter for a search that will sort
// results by the @timestamp field in descending order.
func SortTimestampDesc() map[string]types.FieldSort {
	return map[string]types.FieldSort{
		"@timestamp": {
			Order: &sortorder.Desc,
		},
	}
}
