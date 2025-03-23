// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package api

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/joshuar/go-feed-me/internal/validation"
)

var (
	ErrParseFilters = errors.New("error parsing filters")
)

// Sort by last updated, newest->oldest.
var SortLastUpdatedDesc = Sort{SortBy: SortByLastUpdated, SortOrder: SortOrderDesc}

// Sort by last updated, oldest->newest.
var SortLastUpdatedAsc = Sort{SortBy: SortByLastUpdated, SortOrder: SortOrderAsc}

// Sort by unread count, highest->lowest.
var SortUnreadCountDesc = Sort{SortBy: SortByUnreadCount, SortOrder: SortOrderDesc}

// Sort by unread count, lowest->highest.
var SortUnreadCountAsc = Sort{SortBy: SortByUnreadCount, SortOrder: SortOrderAsc}

const (
	MaxUserCount = 20
	MinUserCount = 1

	// DefaultCount is to show 10 objects.
	DefaultCount = 10
	// DefaultView is to show unread objects.
	DefaultView = ViewUnread
	// DefaultSortBy is to sort on updated.
	DefaultSortBy = SortByLastUpdated
	// DefaultSortOrder is to sort newest->oldest.
	DefaultSortOrder = SortOrderDesc
	// DefaultSince is maximum duration (approx 290 years).
	DefaultSince = math.MaxInt64
)

// Defines values for Param.
const (
	ParamCategories = "categories"
	ParamCount      = "count"
	ParamFeeds      = "feeds"
	ParamItems      = "items"
	ParamPagination = "pagination"
	ParamSince      = "since"
	ParamSortBy     = "sort_by"
	ParamSortOrder  = "sort_order"
	ParamView       = "view"
)

// FiltersValidation is a custom struct-level validation function for Filters.
// In this case, we validate that either a list of Feeds or Items has been
// provided, and fail validation if both have been provided.
func FiltersValidation(sl validator.StructLevel) {
	filters := sl.Current().Interface().(Filters)
	if len(filters.Feeds) > 0 && len(filters.Items) > 0 {
		sl.ReportError(filters.Feeds, "feeds", "Feeds", "feedsoritems", "")
		sl.ReportError(filters.Items, "items", "Items", "feedsoritems", "")
	}
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

// MarshalJSON ensures Filters satisfies the json.Marshaler interface.
func (f *Filters) MarshalJSON() ([]byte, error) {
	return json.Marshal(f)
}

// MarshalJSON ensures Filters satisfies the json.Unmarshaler interface.
func (f *Filters) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, f)
}

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

// Generate will create a Filters object from the given object (usually
// parameters from a handler).
func (f *Filters) Generate(params any) error {
	filters := &Filters{}
	// Marshal params to JSON.
	data, err := json.Marshal(params)
	if err != nil {
		return WrapError(err, "api", "unable to marshal params")
	}
	// Unmarshal JSON to filters.
	err = filters.UnmarshalJSON(data)
	if err != nil {
		return WrapError(err, "api", "unable to unmarshal filters from params")
	}

	valid, err := filters.Valid()
	if !valid || err != nil {
		return WrapError(err, "api", "invalid feed filters")
	}

	return nil
}

// ToQueryParams returns the filters as a url.Values object.
func (f *Filters) ToQueryParams() url.Values {
	params := make(url.Values)

	if len(f.Feeds) > 0 {
		params.Set(ParamFeeds, strings.Join(f.Feeds, ","))
	}

	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}

	params.Set(ParamSortBy, string(f.SortBy))
	params.Set(ParamSortOrder, string(f.SortOrder))
	params.Set(ParamView, string(f.View))
	params.Set(ParamCount, strconv.Itoa(f.Count))

	return params
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
	if value < MinUserCount || value > MaxUserCount {
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

// setValidSince parses string representing a duration and returns the valid
// Since value it represents. If the value cannot be parsed, the default Since
// value is returned.
func setValidSince(value string) Since {
	since, err := time.ParseDuration(value)
	if err != nil {
		return DefaultSince
	}

	return since
}
