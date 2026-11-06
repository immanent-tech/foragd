// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/immanent-tech/go-base/validation"
)

const (
	// DefaultListResultsCount is how many objects can be fetched in a single request when fetching for a list page.
	DefaultListResultsCount = 9
	// defaultView is to show unread objects.
	defaultView = ViewUnread
	// defaultSort is newest first.
	defaultSort = SortNewestFirst
	// defaultMark is Mark Read.
	defaultMark = MarkRead
)

func (s Sort) String() string {
	switch s {
	case SortLeastUnread:
		return "Least Unread"
	case SortMostUnread:
		return "Most Unread"
	case SortNewestFirst:
		return "Newest First"
	case SortOldestFirst:
		return "Oldest First"
	case SortMostRelevant:
		return "Most Relevant"
	default:
		return "Unknown"
	}
}

var validSort = map[Sort]bool{
	SortLeastUnread:  true,
	SortMostRelevant: true,
	SortMostUnread:   true,
	SortNewestFirst:  true,
	SortOldestFirst:  true,
}

var validMark = map[Mark]bool{
	MarkRead:   true,
	MarkUnread: true,
}

var validView = map[View]bool{
	ViewAll:       true,
	ViewUnread:    true,
	ViewRead:      true,
	ViewFavorites: true,
}

// ParseListFilters creates a new ListFilters from the given URL query values.
func ParseListFilters(query url.Values) *ListFilters {
	filters := NewListFilters()
	if v := query.Get("sort"); validSort[Sort(v)] {
		filters.Sort = Sort(v)
	}
	if v := query.Get("view"); validView[View(v)] {
		filters.View = View(v)
	}
	if v, err := strconv.Atoi(query.Get("from")); err == nil && v > 0 {
		filters.From = v
	}
	if c := query.Get("category"); c != "" {
		filters.Category = &c
	}
	if c := query.Get("language"); c != "" {
		filters.Language = &c
	}
	if a := query.Get("search_after"); a != "" {
		filters.SearchAfter = &a
	}
	if s := query.Get("subscriptions"); s != "" {
		filters.Subscriptions = strings.Split(s, ",")
	}
	return filters
}

// NewListFilters creates a new ListFilters with default values.
func NewListFilters() *ListFilters {
	return &ListFilters{
		Sort: defaultSort,
		From: 0,
		View: defaultView,
	}
}

func (f ListFilters) GetView() View {
	return f.View
}

// GetFrom returns the index in a set of results from where to start retrieving results.
func (f ListFilters) GetFrom() int {
	return f.From
}

// GetUpTo returns the index in a set of results up to which results should be retrieved.
func (f ListFilters) GetUpTo() int {
	if f.Count != nil && *f.Count > 0 {
		return *f.Count
	}
	return f.From + DefaultListResultsCount
}

func (f ListFilters) GetSort() Sort {
	return f.Sort
}

func (f ListFilters) GetLanguage() *Language {
	if f.Language != nil {
		return f.Language
	}
	return nil
}

func (f ListFilters) GetCategories() Categories {
	if f.Category != nil {
		return []Category{*f.Category}
	}
	return nil
}

func (f ListFilters) GetSubscriptions() []SubscriptionID {
	return f.Subscriptions
}

// Sanitise performs sanitisation of the filter values to ensure correctness.
func (f *ListFilters) Sanitise() error {
	if f == nil {
		f = NewListFilters()
	}
	if !validSort[f.Sort] {
		f.Sort = defaultSort
	}
	if !validView[f.View] {
		f.View = defaultView
	}
	if f.From < 0 {
		f.From = 0
	}
	if f.Category != nil {
		f.Category = new(validation.SanitizeInput(*f.Category))
	}
	if f.Language != nil {
		f.Language = new(validation.SanitizeInput(*f.Language))
	}
	if len(f.Subscriptions) > 0 {
		slices.Sort(f.Subscriptions)
		f.Subscriptions = slices.Compact(f.Subscriptions)
	}
	if f.SearchAfter != nil {
		if pagination, err := url.QueryUnescape(*f.SearchAfter); err != nil {
			f.SearchAfter = nil
		} else {
			f.SearchAfter = &pagination
		}
	}
	return nil
}

// Validate will return a boolean indicating whether the filters are valid and a non-nil error with details if not.
func (f *ListFilters) Validate() error {
	if f == nil {
		return nil
	}
	if err := validation.Validate.Struct(f); err != nil {
		return fmt.Errorf("filters are invalid: %w", err)
	}
	return nil
}

// Encode returns the query string encoded value of the filters.
func (f ListFilters) Encode() string {
	query := url.Values{}
	query.Set("sort", string(f.Sort))
	query.Set("view", string(f.View))
	query.Set("from", strconv.Itoa(f.From))
	if f.Category != nil {
		query.Set("category", *f.Category)
	}
	if f.Language != nil {
		query.Set("language", *f.Language)
	}
	if len(f.Subscriptions) > 0 {
		query.Set("subscriptions", strings.Join(f.Subscriptions, ","))
	}
	if f.SearchAfter != nil {
		query.Set("search_after", url.QueryEscape(*f.SearchAfter))
	}
	return query.Encode()
}
