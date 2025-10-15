// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/gob"
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/immanent-tech/foragd/server/forms"
	"github.com/immanent-tech/foragd/validation"
)

func init() {
	gob.Register(ListDisplayFilters{})
}

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

// ID returns a string that can be used as an id for the sort.
func (s *Sort) ID() string {
	return "sort-" + string(s.SortBy) + "-" + string(s.SortOrder)
}

func (s Sort) IsEqual(value Sort) bool {
	return s.SortBy == value.SortBy && s.SortOrder == value.SortOrder
}

// Valid checks whether the Sort options are valid values.
func (s *Sort) Valid() bool {
	valid, err := validation.ValidateStruct(s)
	if !valid || err != nil {
		return false
	}
	return true
}

// Filters represents either Subscription or Article filters.
type Filters interface {
	Valid() (bool, error)
	GetSort() Sort
	GetCount() int
	GetView() View
	GetCategories() []Category
	Values() map[string]string
	QueryString() string
}

func NewListDisplayFilters() ListDisplayFilters {
	return ListDisplayFilters{
		SortBy:    SortByLastUpdated,
		SortOrder: SortOrderDesc,
		Count:     DefaultCount,
		View:      DefaultView,
	}
}

func (f *ListDisplayFilters) GetSubscriptions() []SubscriptionID {
	return f.Subscriptions
}

func (f *ListDisplayFilters) Sanitise() error {
	if f == nil {
		newFilters := NewListDisplayFilters()
		f = &newFilters
		return nil
	}
	// Set required filters to valid values as necessary.
	f.SortBy = setValidSortBy(f.SortBy)
	f.SortOrder = setValidSortOrder(f.SortOrder)
	f.Count = setValidCount(f.Count)
	f.View = setValidView(f.View)
	return nil
}

// Valid will return a boolean indicating whether the filters are valid and a
// non-nil error with details if not.
func (f *ListDisplayFilters) Valid() (bool, error) {
	if f == nil {
		return false, forms.ErrNoFormData
	}
	return validation.ValidateStruct(f)
}

// Sort returns the Sort object for the Filters.
func (f *ListDisplayFilters) GetSort() Sort {
	return Sort{
		SortBy:    f.SortBy,
		SortOrder: f.SortOrder,
	}
}

// GetCount returns the count value (encoded as a string in the filters) as an int.
func (f *ListDisplayFilters) GetCount() int {
	value, err := strconv.Atoi(f.Count)
	if err != nil {
		return 10
	}
	return value
}

func (f *ListDisplayFilters) GetView() View {
	return f.View
}

func (f *ListDisplayFilters) GetCategories() []Category {
	return f.Categories
}

func (f *ListDisplayFilters) QueryParams() url.Values {
	params := make(url.Values)
	if len(f.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(f.Subscriptions, ","))
	}
	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}
	params.Set(ParamSortBy, string(f.SortBy))
	params.Set(ParamSortOrder, string(f.SortOrder))
	params.Set(ParamView, string(f.View))
	params.Set(ParamCount, f.Count)
	return params
}

func (f *ListDisplayFilters) QueryString() string {
	return f.QueryParams().Encode()
}

func (f *ListDisplayFilters) Values() map[string]string {
	params := make(map[string]string)
	if len(f.Subscriptions) > 0 {
		params[ParamSubscriptions] = strings.Join(f.Subscriptions, ",")
	}
	if len(f.Categories) > 0 {
		params[ParamCategories] = strings.Join(f.Categories, ",")
	}
	params[ParamSortBy] = string(f.SortBy)
	params[ParamSortOrder] = string(f.SortOrder)
	params[ParamView] = string(f.View)
	params[ParamCount] = f.Count
	return params
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
