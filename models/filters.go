// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/joshuar/go-feed-me/components/validation"
)

var ErrParseFilters = errors.New("error parsing filters")

var (
	// Sort by last updated, newest->oldest.
	SortLastUpdatedDesc = Sort{SortBy: SortByLastUpdated, SortOrder: SortOrderDesc}
	// Sort by last updated, oldest->newest.
	SortLastUpdatedAsc = Sort{SortBy: SortByLastUpdated, SortOrder: SortOrderAsc}
	// Sort by unread count, highest->lowest.
	SortUnreadCountDesc = Sort{SortBy: SortByUnreadCount, SortOrder: SortOrderDesc}
	// Sort by unread count, lowest->highest.
	SortUnreadCountAsc = Sort{SortBy: SortByUnreadCount, SortOrder: SortOrderAsc}
)

const (
	// MaxUserCount is the maximum number of results a user can retrieve at a single time.
	MaxUserCount = 20
	// MinUserCount is the mininum number of results a user can retrieve at a single time.
	MinUserCount = 1
	// DefaultCount is to show 10 objects.
	DefaultCount = "10"
	// DefaultView is to show unread objects.
	DefaultView = ViewUnread
	// DefaultSortBy is to sort on updated.
	DefaultSortBy = SortByLastUpdated
	// DefaultSortOrder is to sort newest->oldest.
	DefaultSortOrder = SortOrderDesc
	// DefaultSince is maximum duration (approx 290 years).
	DefaultSince = math.MaxInt64
)

// FiltersValidation is a custom struct-level validation function for Filters.
// In this case, we validate that either a list of Feeds or Items has been
// provided, and fail validation if both have been provided.
func FiltersValidation(sl validator.StructLevel) {
	// filters := sl.Current().Interface().(Filters)
	// if len(filters.Feeds) > 0 && len(filters.Items) > 0 {
	// 	sl.ReportError(filters.Feeds, "feeds", "Feeds", "feedsoritems", "")
	// 	sl.ReportError(filters.Items, "items", "Items", "feedsoritems", "")
	// }
}

// Valid will return a boolean indicating whether the filters are valid and a
// non-nil error with details if not.
func (f *Filters) Valid() (bool, error) {
	// Register custom struct-level validation function.
	validation.AddStructValidationFunc(FiltersValidation, Filters{})
	// Set required filters to valid values as necessary.
	f.SortBy = setValidSortBy(f.SortBy)
	f.SortOrder = setValidSortOrder(f.SortOrder)
	f.Count = setValidCount(f.Count)
	f.View = setValidView(f.View)
	// Validate struct.
	return validation.ValidateStruct(f)
}

// // MarshalJSON ensures Filters satisfies the json.Marshaler interface.
// func (f *Filters) MarshalJSON() ([]byte, error) {
// 	return json.Marshal(f)
// }

// // MarshalJSON ensures Filters satisfies the json.Unmarshaler interface.
// func (f *Filters) UnmarshalJSON(data []byte) error {
// 	return json.Unmarshal(data, f)
// }

// ViewRead returns a boolean indicating whether the Filters are set to view
// read items.
func (f *Filters) ViewRead() bool {
	return f.View == ViewRead
}

// ViewUnread returns a boolean indicating whether the Filters are set to view
// unread items.
func (f *Filters) ViewUnread() bool {
	return f.View == ViewUnread
}

// ViewAll returns a boolean indicating whether the Filters are set to view
// all (read and unread) items.
func (f *Filters) ViewAll() bool {
	return f.View == ViewAll
}

// Sort returns the Sort object for the Filters.
func (f *Filters) Sort() Sort {
	return Sort{
		SortBy:    f.SortBy,
		SortOrder: f.SortOrder,
	}
}

// CountAsInt returns the count value (encoded as a string in the filters) as an int.
func (f *Filters) CountAsInt() int {
	value, err := strconv.Atoi(f.Count)
	if err != nil {
		return 10
	}
	return value
}

// ToQueryParams returns the filters as a url.Values object.
func (f *Filters) ToQueryParams() url.Values {
	params := make(url.Values)

	// if len(f.Feeds) > 0 {
	// 	params.Set(ParamFeeds, strings.Join(f.Feeds, ","))
	// }

	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}

	if f.Pagination != nil {
		params.Set(ParamPagination, *f.Pagination)
	}

	params.Set(ParamSortBy, string(f.SortBy))
	params.Set(ParamSortOrder, string(f.SortOrder))
	params.Set(ParamView, string(f.View))
	params.Set(ParamCount, f.Count)

	return params
}

// HasCategory returns a boolean indicating whether the given category is set in the filters. If true, a positive
// integer will also be returned indicating the index value of the category in the slice.
func (f *Filters) HasCategory(category Category) (bool, int) {
	idx := slices.IndexFunc(f.Categories, func(c Category) bool { return c == category })
	if idx != -1 {
		return true, idx
	}
	return false, idx
}

// AddCategory adds the given category to the filters. Duplicate values will not be added.
func (f *Filters) AddCategory(category Category) {
	if found, _ := f.HasCategory(category); !found {
		f.Categories = append(f.Categories, category)
	}
}

// RemoveCategory removes the given category from the filters.
func (f *Filters) RemoveCategory(category Category) {
	if found, idx := f.HasCategory(category); found {
		f.Categories = slices.Delete(f.Categories, idx, idx+1)
	}
}

// HasPagination returns a boolean indicating whether there are pagination values.
func (f *Filters) HasPagination() bool {
	return f.Pagination != nil
}

// NewFiltersFromParams creates new filters with values extracted from the given request params.
func NewFiltersFromParams(params any) (*Filters, error) {
	filters := NewFilters()
	// Marshal params to JSON.
	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal params: %w", err)
	}
	// Unmarshal JSON to filters.
	err = json.Unmarshal(data, filters)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal params: %w", err)
	}

	valid, err := filters.Valid()
	if !valid || err != nil {
		return nil, fmt.Errorf("invalid filters: %w", err)
	}

	return filters, nil
}

// NewFilters creates a new Filters object with default values.
func NewFilters() *Filters {
	return &Filters{
		Count:     DefaultCount,
		View:      DefaultView,
		SortBy:    DefaultSortBy,
		SortOrder: DefaultSortOrder,
	}
}

func (sb SortBy) String() string {
	switch sb {
	case SortByLastUpdated:
		return "Last Updated"
	case SortByUnreadCount:
		return "Unread Count"
	default:
		return ""
	}
}

func (so SortOrder) String() string {
	switch so {
	case SortOrderAsc:
		return "Asc"
	case SortOrderDesc:
		return "Desc"
	default:
		return ""
	}
}

func (s *Sort) String() string {
	return s.SortBy.String() + ": " + s.SortOrder.String()
}

// Valid checks whether the Sort options are valid values.
func (s *Sort) Valid() bool {
	valid, err := validation.ValidateStruct(s)
	if !valid || err != nil {
		return false
	}
	return true
}

// setValidSortBy takes a string value and returns the SortBy value it
// represents. If the string is not a valid SortBy value, the default SortBy
// value is returned.
func setValidSortBy(value SortBy) SortBy {
	switch value {
	case SortByUnreadCount:
		return value
	case SortByLastUpdated:
		return value
	default:
		return DefaultSortBy
	}
}

// setValidSortOrder takes a string value and returns the SortOrder value it
// represents. If the string is not a valid SortOrder value, the default SortOrder
// value is returned.
func setValidSortOrder(value SortOrder) SortOrder {
	switch value {
	case SortOrderAsc:
		return value
	case SortOrderDesc:
		return value
	default:
		return DefaultSortOrder
	}
}

// setValidCount takes a value representing a count and returns a valid Count it
// represents. If the value is not a valid Count, the default Count is
// returned.
func setValidCount(value Count) Count {
	numeric, err := strconv.Atoi(value)
	if err != nil {
		return DefaultCount
	}
	if numeric < MinUserCount || numeric > MaxUserCount {
		return DefaultCount
	}
	return value
}

// setValidView takes a string representing a View and returns a valid View it
// represents. If the value is not a valid View, the default View is
// returned.
func setValidView(value View) View {
	switch value {
	case ViewAll:
		return ViewAll
	case ViewRead:
		return ViewRead
	case ViewUnread:
		return ViewUnread
	default:
		return DefaultView
	}
}
