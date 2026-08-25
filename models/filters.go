// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/immanent-tech/go-base/validation"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/server/session"
)

func init() {
	gob.Register(ListFilters{})
	gob.Register(ListFilters2{})
}

var ErrNoFilters = NewAPIError(http.StatusBadRequest, errors.New("no filters found"))

const (
	// minUserCount is the minimum number of results a user can retrieve at a single time.
	minUserCount = 1
	// DefaultCount is to show 9 objects.
	DefaultCount = 9
	// MaxCount is the maximum number of objects to fetch at once.
	MaxCount = 90
	// MaxUpTo is the maximum number of objects to fetch when restoring the page.
	MaxUpTo = 1000
	// defaultView is to show unread objects.
	defaultView = ViewUnread
)

// defaultSort is newest first.
var defaultSort = SortNewestFirst

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

// NewListDisplayFilters creates a new set of display filters with sensible defaults.
func NewListDisplayFilters() ListFilters {
	return ListFilters{
		Sort:  defaultSort,
		Count: DefaultCount,
		View:  defaultView,
	}
}

// GetSubscriptions retrieves any subscription filters.
func (f *ListFilters) GetSubscriptions() []SubscriptionID {
	return f.Subscriptions
}

// Sanitise performs sanitisation of the filter values to ensure correctness.
func (f *ListFilters) Sanitise() error {
	if f == nil {
		return ErrNoFilters
	}
	if len(f.Subscriptions) > 0 {
		f.Subscriptions = slices.Compact(f.Subscriptions)
	}
	// Set required filters to valid values as necessary.
	f.Sort = setValidSort(f.Sort)
	f.Count = setValidCount(f.Count)
	f.View = setValidView(f.View)
	return nil
}

// Valid will return a boolean indicating whether the filters are valid and a
// non-nil error with details if not.
func (f *ListFilters) Valid() error {
	if f == nil {
		return ErrNoFilters
	}
	if err := validation.Validate.Struct(f); err != nil {
		return fmt.Errorf("filters are invalid: %w", err)
	}
	return nil
}

// GetSort returns the Sort object for the Filters.
func (f *ListFilters) GetSort() Sort {
	return f.Sort
}

// GetCount returns the count value (encoded as a string in the filters) as an int.
func (f *ListFilters) GetCount() int {
	return f.Count
}

// GetView returns the view filter.
func (f *ListFilters) GetView() View {
	return f.View
}

// GetCategories returns any category filters.
func (f *ListFilters) GetCategories() Categories {
	return f.Categories
}

// QueryParams converts the filters into query parameters.
func (f *ListFilters) QueryParams() url.Values {
	params := make(url.Values)
	if len(f.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(f.Subscriptions, ","))
	}
	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}
	params.Set(ParamSort, string(f.GetSort()))
	params.Set(ParamView, string(f.GetView()))
	params.Set(ParamCount, strconv.Itoa(f.Count))
	return params
}

// QueryString converts the filters into a string that can be appended to a URL to represent the filters.
func (f *ListFilters) QueryString() string {
	return f.QueryParams().Encode()
}

// Values converts the filters into a map[string]string object, that can be further manipulated before being (most
// likely) used as the value of hx-vals in a HTMX request.
func (f *ListFilters) Values() map[string]any {
	params := make(map[string]any)
	if len(f.Subscriptions) > 0 {
		params[ParamSubscriptions] = f.Subscriptions
	}
	if len(f.Categories) > 0 {
		params[ParamCategories] = strings.Join(f.Categories, ",")
	}
	params[ParamSort] = string(f.GetSort())
	params[ParamView] = string(f.GetView())
	params[ParamCount] = f.Count
	return params
}

// setValidSortBy takes a string value and returns the SortBy value it
// represents. If the string is not a valid SortBy value, the default SortBy
// value is returned.
func setValidSort(value Sort) Sort {
	switch value {
	case SortLeastUnread, SortMostUnread, SortNewestFirst, SortOldestFirst, SortMostRelevant:
		return value
	default:
		return defaultSort
	}
}

// setValidCount takes a value representing a count and returns a valid Count it represents. If the value is not a valid
// Count, the default Count is returned.
func setValidCount(value int) int {
	if value < minUserCount {
		return DefaultCount
	}
	return value
}

// setValidView takes a string representing a View and returns a valid View it represents. If the value is not a valid
// View, the default View is returned.
func setValidView(value View) View {
	switch value {
	case ViewFavorites:
		return ViewFavorites
	case ViewAll:
		return ViewAll
	case ViewRead:
		return ViewRead
	case ViewUnread:
		return ViewUnread
	default:
		return defaultView
	}
}

// setValidMark takes a string representing a Mark and returns a valid Mark it represents. If the value is not a valid
// Mark, it returns MarkRead.
func setValidMark(value Mark) Mark {
	switch value {
	case MarkRead:
		return MarkRead
	case MarkUnread:
		return MarkUnread
	default:
		return MarkRead
	}
}

var validSort = map[Sort]bool{
	SortNewestFirst:  true,
	SortOldestFirst:  true,
	SortMostUnread:   true,
	SortLeastUnread:  true,
	SortMostRelevant: true,
}
var validView = map[View]bool{ViewAll: true, ViewUnread: true, ViewRead: true, ViewFavorites: true}

func NewListFilters() *ListFilters2 {
	return &ListFilters2{
		Sort:  defaultSort,
		Count: DefaultCount,
		View:  defaultView,
	}
}

// Sanitise performs sanitisation of the filter values to ensure correctness.
func (f *ListFilters2) Sanitise() error {
	if f == nil {
		f = NewListFilters()
	}
	if !validSort[f.Sort] {
		f.Sort = defaultSort
	}
	if !validView[f.View] {
		f.View = defaultView
	}
	if f.Count < 0 || f.Count > MaxCount {
		f.Count = MaxCount
	}
	if f.UpTo != nil {
		switch {
		case *f.UpTo < 0:
			f.UpTo = nil
		case *f.UpTo > MaxUpTo:
			f.UpTo = new(MaxUpTo)
		}
	}
	if f.From != nil && *f.From < 0 {
		f.From = new(0)
	}
	return nil
}

// Valid will return a boolean indicating whether the filters are valid and a
// non-nil error with details if not.
func (f *ListFilters2) Valid() error {
	switch {
	case f == nil:
		return fmt.Errorf("invalid filters: %w", ErrNoFilters)
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

func ParseListFilters(query url.Values) *ListFilters2 {
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
		filters.SearchAfter = &a
	}
	return filters
}

// Encode returns the query string encoded value of the filters.
func (f ListFilters2) Encode() string {
	query := url.Values{}
	query.Set("sort", string(f.Sort))
	query.Set("view", string(f.View))
	query.Set("count", strconv.Itoa(f.Count))
	if f.Category != nil {
		query.Set("category", *f.Category)
	}
	if f.From != nil {
		query.Set("from", strconv.Itoa(*f.From))
	}
	if f.UpTo != nil {
		query.Set("upto", strconv.Itoa(*f.UpTo))
	}
	if f.SearchAfter != nil {
		query.Set("search_after", *f.SearchAfter)
	}
	return query.Encode()
}

const filtersCtxKey contextKey = "listFilters"

func ListFiltersToCtx(ctx context.Context, filters *ListFilters2) context.Context {
	return context.WithValue(ctx, filtersCtxKey, *filters)
}

func ListFiltersFromCtx(ctx context.Context) *ListFilters2 {
	if filters, ok := ctx.Value(filtersCtxKey).(ListFilters2); ok {
		return &filters
	}
	slogctx.Warn(ctx, "No filters in context. Returning new filters.")
	return NewListFilters()
}

func ListFiltersToSession(ctx context.Context, filters *ListFilters2) {
	if err := session.Save(ctx, string(filtersCtxKey), *filters); err != nil {
		slogctx.Warn(ctx, "Unable to save filters to session.", slog.Any("error", err))
	}
}

func ListFiltersFromSession(ctx context.Context) *ListFilters2 {
	filters, err := session.Restore[ListFilters2](ctx, string(filtersCtxKey))
	if err != nil {
		slogctx.Warn(ctx, "Unable to restore filters from session. Using defaults.", slog.Any("error", err))
		return NewListFilters()
	}
	return &filters

}
