// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"slices"
	"strconv"

	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/refresh"

	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/providers/elastic/retriever"
)

// Option is a functional option for a request of type T.
type Option[T any] func(T) T

type supportsRefresh[T any] interface {
	Refresh(value refresh.Refresh) T
}

// WithRefresh option sets the refresh value on the request.
func WithRefresh[T supportsRefresh[T]](value refresh.Refresh) Option[T] {
	return func(t T) T {
		return t.Refresh(value)
	}
}

type supportsSeqNo[T any] interface {
	IfSeqNo(string) T
}

// WithSeqNo option specifies the sequence no of the doc on which to operate.
func WithSeqNo[T supportsSeqNo[T]](seqno int64) Option[T] {
	v := strconv.FormatInt(seqno, 10)
	return func(t T) T {
		return t.IfSeqNo(v)
	}
}

type supportsPrimaryTerm[T any] interface {
	IfPrimaryTerm(string) T
}

// WithPrimaryTerm option to specifies the primary term of the doc on which to operate.
func WithPrimaryTerm[T supportsPrimaryTerm[T]](term int64) Option[T] {
	v := strconv.FormatInt(term, 10)
	return func(t T) T {
		return t.IfPrimaryTerm(v)
	}
}

type supportsHeader[T any] interface {
	Header(key string, value string) T
}

// WithHeader option sets a HTTP header on the request.
func WithHeader[T supportsHeader[T]](key, value string) Option[T] {
	return func(t T) T {
		return t.Header(key, value)
	}
}

type supportsIndex[T any] interface {
	Index(index string) T
}

// WithIndex option sets the index the request will operate on.
func WithIndex[T supportsIndex[T]](index string) Option[T] {
	return func(t T) T {
		return t.Index(index)
	}
}

type supportsQuery[T any] interface {
	Query(query types.QueryVariant) T
}

// WithQuery option sets a query as a part of the request.
func WithQuery[T supportsQuery[T]](options ...query.Option) Option[T] {
	q := query.Build(options...)
	return func(t T) T {
		return t.Query(q)
	}
}

type supportsRetriever[T any] interface {
	Retriever(retriever types.RetrieverContainerVariant) T
}

// WithRetriever option sets a retriever as part of the request.
func WithRetriever[T supportsRetriever[T]](options ...retriever.Option) Option[T] {
	r := &types.RetrieverContainer{}
	for option := range slices.Values(options) {
		option(r)
	}
	return func(t T) T {
		return t.Retriever(r)
	}
}

type supportsAggregations[T any] interface {
	AddAggregation(key string, value types.AggregationsVariant) T
	Aggregations(aggregations map[string]types.Aggregations) T
}

// WithAggregations option sets the aggregations to perform with the request.
func WithAggregations[T supportsAggregations[T]](aggregations map[string]types.Aggregations) Option[T] {
	return func(t T) T {
		return t.Aggregations(aggregations)
	}
}

// WithAggregation option adds an aggregation to perform with the request.
func WithAggregation[T supportsAggregations[T]](key string, value types.AggregationsVariant) Option[T] {
	return func(t T) T {
		return t.AddAggregation(key, value)
	}
}

type supportsFrom[T any] interface {
	From(from int) T
}

// From option sets the pagination point within the results.
func WithFrom[T supportsFrom[T]](from int) Option[T] {
	return func(t T) T {
		return t.From(from)
	}
}

type supportsSize[T any] interface {
	Size(size int) T
}

// WithSize option sets the number of results that will be returned.
func WithSize[T supportsSize[T]](size int) Option[T] {
	return func(t T) T {
		return t.Size(size)
	}
}

type supportsSort[T any] interface {
	Sort(sorts ...types.SortCombinationsVariant) T
}

// WithSort option sets how the results will be sorted.
func WithSort[T supportsSort[T]](sorts ...types.SortCombinationsVariant) Option[T] {
	return func(t T) T {
		return t.Sort(sorts...)
	}
}

// WithDocSorting option is a convenience option to sort the results by doc id. This option is useful when fetching all
// results or deep pagination where sort order is irrelevant.
func WithDocSorting[T supportsSort[T]]() Option[T] {
	return func(t T) T {
		return t.Sort(&types.SortOptions{Doc_: types.NewScoreSort()})
	}
}

type supportsSearchAfter[T any] interface {
	SearchAfter(sortresults ...types.FieldValueVariant) T
}

// WithSearchAfter option sets the point after which results should be fetched.
func WithSearchAfter[T supportsSearchAfter[T]](sortresults ...types.FieldValueVariant) Option[T] {
	return func(t T) T {
		return t.SearchAfter(sortresults...)
	}
}

// TrackHits is a boolean indicating whether to track total hits.
type TrackHits bool

func (t TrackHits) TrackHitsCaster() *types.TrackHits {
	value := types.TrackHits(t)
	return &value
}

type supportsTrackHits[T any] interface {
	TrackTotalHits(trackhits types.TrackHitsVariant) T
}

// WithTrackHits option sets whether total hits should be tracked.
func WithTrackHits[T supportsTrackHits[T]](value TrackHits) Option[T] {
	return func(t T) T {
		return t.TrackTotalHits(value)
	}
}

type supportsCollapse[T any] interface {
	Collapse(collapse types.FieldCollapseVariant) T
}

// WithCollapse option sets that duplicate results for the given field should be collapsed down to the first result.
func WithCollapse[T supportsCollapse[T]](field string) Option[T] {
	return func(t T) T {
		return t.Collapse(&types.FieldCollapse{Field: field})
	}
}

type supportsIDs[T any] interface {
	Ids(ids ...string) T
}

// WithDocIDs option sets the doc IDs to retrieve.
func WithDocIDs[T supportsIDs[T]](ids ...string) Option[T] {
	return func(t T) T {
		return t.Ids(ids...)
	}
}

type supportsRetryOnConflict[T any] interface {
	RetryOnConflict(retryonconflict int) T
}

// WithRetryOnConflict option sets the number of times the request will be retried if there is a conflict.
func WithRetryOnConflict[T supportsRetryOnConflict[T]](retries int) Option[T] {
	return func(t T) T {
		return t.RetryOnConflict(retries)
	}
}

type supportsDocAsUpsert[T any] interface {
	DocAsUpsert(docasupsert bool) T
}

// WithDocAsUpsert option specifies that the doc should be added if it does not already exist. Normally, the request
// would not add a document if none already exist.
func WithDocAsUpsert[T supportsDocAsUpsert[T]](value bool) Option[T] {
	return func(t T) T {
		return t.DocAsUpsert(value)
	}
}

type supportsSeqNoPrimaryTerm[T any] interface {
	SeqNoPrimaryTerm(value bool) T
}

// WithSeqNoPrimaryTerm option specifies that the results should return the sequence no and primary term of each hit.
func WithSeqNoPrimaryTerm[T supportsSeqNoPrimaryTerm[T]](value bool) Option[T] {
	return func(t T) T {
		return t.SeqNoPrimaryTerm(value)
	}
}
