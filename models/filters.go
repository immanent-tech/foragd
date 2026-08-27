// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/immanent-tech/go-base/validation"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/server/session"
)

const (
	// maxCount is the maximum number of objects to fetch at once.
	maxCount = 90
	// maxUpTo is the maximum number of objects to fetch when restoring the page.
	maxUpTo = 1000
	// defaultCount is to show 9 objects.
	defaultCount = 9
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
	if v, err := strconv.Atoi(query.Get("count")); err == nil && v > 0 {
		filters.Count = v
	}
	if c := query.Get("category"); c != "" {
		filters.Category = &c
	}
	if v, err := strconv.Atoi(query.Get("from")); err == nil && v > 0 {
		filters.From = &v
	}
	if v, err := strconv.Atoi(query.Get("upto")); err == nil && v > 0 {
		filters.UpTo = &v
	}
	if a := query.Get("search_after"); a != "" {
		if pagination, err := url.QueryUnescape(a); err != nil {
			filters.SearchAfter = &pagination
		}
	}
	if s := query.Get("subscriptions"); s != "" {
		filters.Subscriptions = strings.Split(s, ",")
	}
	return filters
}

// NewListFilters creates a new ListFilters with default values.
func NewListFilters() *ListFilters {
	return &ListFilters{
		Sort:  defaultSort,
		Count: defaultCount,
		View:  defaultView,
	}
}

func (f ListFilters) GetView() View {
	return f.View
}

func (f ListFilters) GetCount() int {
	return f.Count
}

func (f ListFilters) GetSort() Sort {
	return f.Sort
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
	if f.Count < 0 || f.Count > maxCount {
		f.Count = maxCount
	}
	if f.Category != nil {
		f.Category = new(validation.SanitizeString(*f.Category))
	}
	if len(f.Subscriptions) > 0 {
		slices.Sort(f.Subscriptions)
		f.Subscriptions = slices.Compact(f.Subscriptions)
	}
	if f.UpTo != nil {
		switch {
		case *f.UpTo < 0:
			f.UpTo = nil
		case *f.UpTo > maxUpTo:
			f.UpTo = new(maxUpTo)
		}
	}
	if f.From != nil && *f.From < 0 {
		f.From = new(0)
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

// Valid will return a boolean indicating whether the filters are valid and a non-nil error with details if not.
func (f ListFilters) Valid() error {
	switch {
	case f.From != nil && (f.UpTo != nil || f.SearchAfter != nil):
		return errors.New("invalid filters: from can only be set if upto or searchAfter is unset")
	case f.SearchAfter != nil && (f.UpTo != nil || f.From != nil):
		return errors.New("invalid filters: searchAfter can only be set if upto or from is unset")
	case f.UpTo != nil && (f.From != nil || f.SearchAfter != nil):
		return errors.New("invalid filters: upto can only be set if from or searchAfter is unset")
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
	query.Set("count", strconv.Itoa(f.Count))
	if f.Category != nil {
		query.Set("category", *f.Category)
	}
	if len(f.Subscriptions) > 0 {
		query.Set("subscriptions", strings.Join(f.Subscriptions, ","))
	}
	if f.From != nil {
		query.Set("from", strconv.Itoa(*f.From))
	}
	if f.UpTo != nil {
		query.Set("upto", strconv.Itoa(*f.UpTo))
	}
	if f.SearchAfter != nil {
		query.Set("search_after", url.QueryEscape(*f.SearchAfter))
	}
	return query.Encode()
}

const listFiltersCtxKey contextKey = "listFilters"
const listCountCtxKey contextKey = "listCount"

// ListFiltersToCtx stores the given list filters in the context.
func ListFiltersToCtx(ctx context.Context, filters *ListFilters) context.Context {
	return context.WithValue(ctx, listFiltersCtxKey, *filters)
}

// ListFiltersFromCtx retrieves the given list filters in the context.
func ListFiltersFromCtx(ctx context.Context) *ListFilters {
	if filters, ok := ctx.Value(listFiltersCtxKey).(ListFilters); ok {
		return &filters
	}
	slogctx.Warn(ctx, "No filters in context. Returning new filters.")
	return NewListFilters()
}

// ListFiltersToSession stores the given list filters in the session. The path is used as a suffix so that filters are
// stored per-route.
func ListFiltersToSession(ctx context.Context, path string, filters *ListFilters) {
	if err := session.Save(ctx, string(listFiltersCtxKey)+path, *filters); err != nil {
		slogctx.Warn(ctx, "Unable to save filters to session.", slog.Any("error", err))
	}
}

// ListFiltersFromSession retrieves the given list filters in the session. The path is used as a suffix so that filters
// are retrieved per-route.
func ListFiltersFromSession(ctx context.Context, path string) *ListFilters {
	filters, err := session.Restore[ListFilters](ctx, string(listFiltersCtxKey)+path)
	if err != nil {
		slogctx.Warn(ctx, "Unable to restore filters from session. Using defaults.", slog.Any("error", err))
		return NewListFilters()
	}
	return &filters
}

// ListCountToSession stores the current count of objects displayed in the list in the session. The path is used as a
// suffix so that the count is stored per-route.
func ListCountToSession(ctx context.Context, path string, count int) {
	if err := session.Save(ctx, string(listCountCtxKey)+path, count); err != nil {
		slogctx.Warn(ctx, "Unable to save list count to session.", slog.Any("error", err))
	}
}

// ListCountFromSession stores the current count of objects displayed in the list in the session. The path is used as a
// suffix so that the count is retrieved per-route.
func ListCountFromSession(ctx context.Context, path string) int {
	count, err := session.Restore[int](ctx, string(listCountCtxKey)+path)
	if err != nil {
		slogctx.Warn(ctx, "Unable to restore list count from session. Using defaults.", slog.Any("error", err))
		return defaultCount
	}
	return count
}
