// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/immanent-tech/go-base/validation"
)

const (
	// DefaultSearchSort is by most relevant.
	DefaultSearchSort = SortMostRelevant
	// defaultSearchTimezone is UTC.
	defaultSearchTimezone = "UTC"
	// DefaultSearchCount is the default number of search results to fetch at once.
	DefaultSearchCount = 15
)

// ParseSearchParams creates a new SearchRequest from the given URL query values.
func ParseSearchParams(query url.Values) *SearchRequest {
	search := NewSearchRequest()
	if v := query.Get("authors"); v != "" {
		search.Authors = &v
	}
	if v := query.Get("categories"); v != "" {
		search.Categories = &v
	}
	if v, err := strconv.Atoi(query.Get("from")); err == nil && v > 0 {
		search.From = &v
	}
	if v, err := strconv.Atoi(query.Get("upto")); err == nil && v > 0 {
		search.UpTo = &v
	}
	if a := query.Get("search_after"); a != "" {
		if pagination, err := url.QueryUnescape(a); err != nil {
			search.SearchAfter = &pagination
		}
	}
	if v := query.Get("published_within"); v != "" {
		search.PublishedWithin = SearchRequestPublishedWithin(v)
	}
	if v := query.Get("sort"); validSort[Sort(v)] {
		search.Sort = Sort(v)
	}
	if v := query.Get("view"); validView[View(v)] {
		search.View = View(v)
	}
	if v := query.Get("subscription_id"); v != "" {
		search.SubscriptionID = &v
	}
	if s := query.Get("subscriptions"); s != "" {
		search.Subscriptions = strings.Split(s, ",")
	}
	if v := query.Get("text"); v != "" {
		search.Text = v
	}
	if v := query.Get("timezone"); v != "" {
		search.Timezone = v
	}
	return search
}

// NewSearchRequest creates a new SearchRequest object with default values. Defaults are search all objects within last
// week, sorted by most relevant.
func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		PublishedWithin: SearchRequestPublishedWithinLastWeek,
		View:            ViewAll,
		Sort:            DefaultSearchSort,
		Timezone:        defaultSearchTimezone,
		Count:           DefaultSearchCount,
	}
}

// GetPagination generates a [Pagination] value from the request.
func (r SearchRequest) GetPagination() *Pagination {
	return &Pagination{From: r.From}
}

// Location generates the appropriate [*time.Location] value based on the request's timezone value. If the request has
// no timezone, it is assumed to be "UTC".
func (r SearchRequest) Location() *time.Location {
	if tz := r.Timezone; tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc, _ := time.LoadLocation("UTC")
			return loc
		}
		return loc
	}
	loc, _ := time.LoadLocation("UTC")
	return loc
}

// Since generates the [time.Time] timestamp in the past from which results should be filtered. If it cannot generate a
// value, a default of the last 7 days is used.
func (r SearchRequest) Since() time.Time {
	defaultSince := time.Now().UTC().Add(-1 * 7 * 24 * time.Hour)
	switch r.PublishedWithin {
	case SearchRequestPublishedWithinLastHour:
		since, err := time.ParseInLocation(time.Layout, time.Now().Add(-time.Hour).Format(time.Layout), r.Location())
		if err != nil {
			return defaultSince
		}
		return since
	case SearchRequestPublishedWithinLast12hours:
		since, err := time.ParseInLocation(time.Layout, time.Now().Add(-12*time.Hour).Format(time.Layout), r.Location())
		if err != nil {
			return defaultSince
		}
		return since
	case SearchRequestPublishedWithinLastDay:
		since, err := time.ParseInLocation(time.Layout, time.Now().Add(-24*time.Hour).Format(time.Layout), r.Location())
		if err != nil {
			return defaultSince
		}
		return since
	case SearchRequestPublishedWithinLastWeek:
		since, err := time.ParseInLocation(
			time.Layout,
			time.Now().Add(-7*24*time.Hour).Format(time.Layout),
			r.Location(),
		)
		if err != nil {
			return defaultSince
		}
		return since
	case SearchRequestPublishedWithinLastMonth:
		since, err := time.ParseInLocation(
			time.Layout,
			time.Now().Add(-30*24*time.Hour).Format(time.Layout),
			r.Location(),
		)
		if err != nil {
			return defaultSince
		}
		return since
	default:
		return defaultSince
	}
}

// Pivot generates a pivot value, which is a duration window which boosts results within this window around the current time.
func (r SearchRequest) Pivot() string {
	switch r.PublishedWithin {
	case SearchRequestPublishedWithinLastHour:
		return "30m"
	case SearchRequestPublishedWithinLast12hours:
		return "6h"
	case SearchRequestPublishedWithinLastDay:
		return "12h"
	case SearchRequestPublishedWithinLastWeek:
		return "3d"
	case SearchRequestPublishedWithinLastMonth:
		return "14d"
	default:
		return "3d"
	}
}

// Valid returns a boolean indicating whether the search request data is valid.
func (r *SearchRequest) Validate() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("search request is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the search request data.
func (r *SearchRequest) Sanitise() error {
	if r == nil {
		return nil
	}
	// Split and sanitise subscriptions field.
	if len(r.Subscriptions) == 1 {
		r.Subscriptions = strings.Split(r.Subscriptions[0], ",")
	}
	for idx, subscription := range r.Subscriptions {
		r.Subscriptions[idx] = validation.SanitizeString(subscription)
	}
	// Sanitise text inputs.
	r.Text = validation.SanitizeString(r.Text)
	if r.Authors != nil {
		cleanAuthors := validation.SanitizeString(*r.Authors)
		r.Authors = &cleanAuthors
	}
	if r.Authors != nil {
		cleanCategories := validation.SanitizeString(*r.Categories)
		r.Categories = &cleanCategories
	}
	// Default timezone is UTC.
	if r.Timezone == "" {
		r.Timezone = defaultSearchTimezone
	}
	// Default published within is last week.
	if r.PublishedWithin == "" {
		r.PublishedWithin = SearchRequestPublishedWithinLastWeek
	}
	// Default view is unread.
	if r.View == "" {
		r.View = ViewUnread
	}
	// Default sort is most relevant.
	if r.Sort == "" {
		r.Sort = DefaultSearchSort
	}
	return nil
}

// Encode returns a string that represents the search as query parameters.
func (r *SearchRequest) Encode() string {
	params := make(url.Values)
	params.Set("text", r.Text)
	if r.Authors != nil {
		params.Set("authors", *r.Authors)
	}
	if r.Categories != nil {
		params.Set("categories", *r.Categories)
	}
	if len(r.Subscriptions) > 0 {
		params.Set("subscriptions", strings.Join(r.Subscriptions, ","))
	}
	params.Set("view", string(r.View))
	params.Set("published_within", string(r.PublishedWithin))
	params.Set("sort", string(r.Sort))
	params.Set("count", strconv.Itoa(r.Count))
	params.Set("timezone", r.Timezone)
	if r.SubscriptionID != nil {
		params.Set("subscription_id", *r.SubscriptionID)
	}
	return params.Encode()
}

// Valid returns a boolean indicating whether the add subscription search filter data is valid.
func (r *AddSubscriptionSearchFilterRequest) Validate() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription search filer is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the add subscription search filter request.
func (r *AddSubscriptionSearchFilterRequest) Sanitise() error {
	r.InputName = validation.SanitizeString(r.InputName)
	r.SubscriptionName = validation.SanitizeString(r.SubscriptionName)
	r.SubscriptionID = validation.SanitizeString(r.SubscriptionID)
	return nil
}
