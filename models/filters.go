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

const (
	// MaxUserCount is the maximum number of results a user can retrieve at a single time.
	MaxUserCount = 20
	// MinUserCount is the mininum number of results a user can retrieve at a single time.
	MinUserCount = 1
	// DefaultCount is to show 10 objects.
	DefaultCount = "10"
	// DefaultView is to show unread objects.
	DefaultView = ViewUnread
	// DefaultSince is maximum duration (approx 290 years).
	DefaultSince = math.MaxInt64
)

// DefaultSort is newest first.
var DefaultSort = SortNewestFirst

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

// NewListDisplayFilters creates a new set of display filters with sensible defaults.
func NewListDisplayFilters() ListDisplayFilters {
	return ListDisplayFilters{
		Sort:  SortNewestFirst,
		Count: DefaultCount,
		View:  DefaultView,
	}
}

// GetSubscriptions retrieves any subscription filters.
func (f *ListDisplayFilters) GetSubscriptions() []SubscriptionID {
	return f.Subscriptions
}

// Sanitise performs sanitisation of the filter values to ensure correctness.
func (f *ListDisplayFilters) Sanitise() error {
	if f == nil {
		newFilters := NewListDisplayFilters()
		f = &newFilters
		return nil
	}
	// Set required filters to valid values as necessary.
	f.Sort = setValidSort(f.Sort)
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

// GetSort returns the Sort object for the Filters.
func (f *ListDisplayFilters) GetSort() Sort {
	return f.Sort
}

// GetCount returns the count value (encoded as a string in the filters) as an int.
func (f *ListDisplayFilters) GetCount() int {
	value, err := strconv.Atoi(f.Count)
	if err != nil {
		return 10
	}
	return value
}

// GetView returns the view filter.
func (f *ListDisplayFilters) GetView() View {
	return f.View
}

// GetCategories returns any category filters.
func (f *ListDisplayFilters) GetCategories() []Category {
	return f.Categories
}

// QueryParams converts the filters into query parameters.
func (f *ListDisplayFilters) QueryParams() url.Values {
	params := make(url.Values)
	if len(f.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(f.Subscriptions, ","))
	}
	if len(f.Categories) > 0 {
		params.Set(ParamCategories, strings.Join(f.Categories, ","))
	}
	params.Set(ParamSort, string(f.Sort))
	params.Set(ParamView, string(f.View))
	params.Set(ParamCount, f.Count)
	return params
}

// QueryString converts the filters into a string that can be appended to a URL to represent the filters.
func (f *ListDisplayFilters) QueryString() string {
	return f.QueryParams().Encode()
}

// Values converts the filters into a map[string]string object, that can be further manipulated before being (most
// likely) used as the value of hx-vals in a HTMX request.
func (f *ListDisplayFilters) Values() map[string]string {
	params := make(map[string]string)
	if len(f.Subscriptions) > 0 {
		params[ParamSubscriptions] = strings.Join(f.Subscriptions, ",")
	}
	if len(f.Categories) > 0 {
		params[ParamCategories] = strings.Join(f.Categories, ",")
	}
	params[ParamSort] = string(f.Sort)
	params[ParamView] = string(f.View)
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
		return DefaultSort
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
