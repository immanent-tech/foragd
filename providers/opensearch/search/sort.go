/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package search

import (
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

const (
	SortOrderAsc  SortOrder = "asc"
	SortOrderDesc SortOrder = "desc"
)

type SortOrder string

func (o SortOrder) String() string {
	return string(o)
}

type Sorting struct {
	Fields map[string]opensearchapi.FieldSort
}

func NewSortOptions(options ...SortOption) *Sorting {
	sorting := &Sorting{}
	for option := range slices.Values(options) {
		option(sorting)
	}
	return sorting
}

func (s Sorting) ToSortCombinations() opensearchapi.SortCombinations {
	if len(s.Fields) > 0 {
		return opensearchapi.NewSortCombinationsFromFieldSortMap(s.Fields)
	}
	return opensearchapi.SortCombinations{}
}

type SortOption func(*Sorting)

func WithSortField(name string, options ...FieldSortOption) SortOption {
	return func(s *Sorting) {
		if s.Fields == nil {
			s.Fields = make(map[string]opensearchapi.FieldSort)
		}
		sort := &opensearchapi.FieldSort{}
		for option := range slices.Values(options) {
			option(sort)
		}
		s.Fields[name] = *sort
	}
}

type FieldSortOption func(*opensearchapi.FieldSort)

func WithFieldSortOrder(order SortOrder) FieldSortOption {
	return func(s *opensearchapi.FieldSort) {
		o := order.String()
		s.Order = &o
	}
}
