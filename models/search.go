// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/providers/elastic/query"
	"github.com/immanent-tech/foragd/validation"
)

// NewSearchRequest creates a new SearchRequest object with default values.
func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		PublishedWithin: SearchRequestPublishedWithinLastWeek,
		View:            ViewUnread,
	}
}

// Valid returns a boolean indicating whether the search request data is valid.
func (r *SearchRequest) Valid() error {
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
		r.Subscriptions[idx] = sanitization.SanitizeString(subscription)
	}
	// Sanitise text inputs.
	r.Text = sanitization.SanitizeString(r.Text)
	r.Authors = sanitization.SanitizeString(r.Authors)
	r.Categories = sanitization.SanitizeString(r.Categories)
	// Default timezone is UTC.
	if r.Timezone == "" {
		r.Timezone = "UTC"
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
		r.Sort = SortMostRelevant
	}
	return nil
}

// Query returns a string that represents the search as query parameters.
func (r *SearchRequest) Query() string {
	return r.params().Encode()
}

// HXVals returns a string that represents the search as hx-vals.
func (r *SearchRequest) HXVals() string {
	data, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(data)
}

func (r *SearchRequest) params() url.Values {
	params := make(url.Values)
	params.Set("text", r.Text)
	if r.Authors != "" {
		params.Set("authors", r.Authors)
	}
	if r.Categories != "" {
		params.Set(ParamCategories, r.Categories)
	}
	if len(r.Subscriptions) > 0 {
		params.Set(ParamSubscriptions, strings.Join(r.Subscriptions, ","))
	}
	params.Set(ParamView, string(r.View))
	params.Set("published_within", string(r.PublishedWithin))
	params.Set(ParamSort, string(r.Sort))
	params.Set("timezone", r.Timezone)
	if r.ID != "" {
		params.Set(ParamSubscriptionID, r.ID)
	}
	return params
}

// Valid returns a boolean indicating whether the add subscription search filter data is valid.
func (r *AddSubscriptionSearchFilterRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription search filer is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the add subscription search filter request.
func (r *AddSubscriptionSearchFilterRequest) Sanitise() error {
	r.InputName = sanitization.SanitizeString(r.InputName)
	r.SubscriptionName = sanitization.SanitizeString(r.SubscriptionName)
	r.SubscriptionID = sanitization.SanitizeString(r.SubscriptionID)
	return nil
}

// Valid returns a boolean indicating whether the add subscription search filter data is valid.
func (r *GetSubscriptionsSuggestionRequest) Valid() error {
	if err := validation.Validate.Struct(r); err != nil {
		return fmt.Errorf("subscription suggestion is invalid: %w", err)
	}
	return nil
}

// Sanitise will sanitise the add subscription search filter request.
func (r *GetSubscriptionsSuggestionRequest) Sanitise() error {
	r.Text = sanitization.SanitizeString(r.Text)
	return nil
}

func SearchResultsClause(search *SearchRequest) query.BoolOption {
	// Must match either: search term in any of the fields, or, matches directly as a search-as-you-type (same as
	// search suggestion).
	return query.Must(
		// Search across title, description and content fields, with preference for match in that order (via field
		// boosting).
		query.SimpleQueryString(search.Text, "", "title^6", "description^3", "content"),
		// Search in categories.
		query.SimpleQueryString(search.Categories, "", "categories"),
		// Search in authors, contributors.
		query.SimpleQueryString(search.Authors, "", "authors", "contributors"),
	)
}

func SearchSuggestionsClause(search *SearchRequest) query.BoolOption {
	// Must match at least one of in title, description, content.
	return query.Must(
		query.Bool(
			query.Should(
				query.SearchAsYouType(search.Text, "title"),
				query.SearchAsYouType(search.Text, "description"),
				query.SearchAsYouType(search.Text, "content"),
			),
		),
	)
}

// BuildSearchResultsQuery generates a query that can be used to fetch appropriate results for a given SearchRequest
// criteria.
func BuildSearchResultsQuery(
	ctx context.Context,
	user *User,
	request *SearchRequest,
	clause query.BoolOption,
) (query.Option, error) {
	// var err error
	var loc *time.Location
	var err error
	if request.Timezone != "" {
		loc, err = time.LoadLocation(request.Timezone)
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	} else {
		loc, err = time.LoadLocation("UTC")
		if err != nil {
			return nil, fmt.Errorf("build search query: load timezone: %w", err)
		}
	}
	var since time.Time
	var pivot string
	switch request.PublishedWithin {
	case SearchRequestPublishedWithinLastHour:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-time.Hour).Format(time.Layout), loc)
		pivot = "30m"
	case SearchRequestPublishedWithinLast12hours:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-12*time.Hour).Format(time.Layout), loc)
		pivot = "6h"
	case SearchRequestPublishedWithinLastDay:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-24*time.Hour).Format(time.Layout), loc)
		pivot = "12h"
	case SearchRequestPublishedWithinLastWeek:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
		pivot = "3d"
	case SearchRequestPublishedWithinLastMonth:
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-30*24*time.Hour).Format(time.Layout), loc)
		pivot = "14d"
	default: // default to one week.
		since, _ = time.ParseInLocation(time.Layout, time.Now().Add(-7*24*time.Hour).Format(time.Layout), loc)
		pivot = "3d"
	}

	subscriptions, err := GetSubscriptions(ctx,
		GetSubscriptionsByIDs(request.Subscriptions...),
	)
	switch {
	case err != nil:
		return nil, fmt.Errorf("build search query: get subscriptions: %w", err)
	case len(subscriptions) == 0:
		return nil, fmt.Errorf("build search query: get subscriptions: %w", ErrNotFound)
	}

	return query.Bool(
		query.WithBoolQueryName("search-results"),
		query.Filter(
			// Must be in the given user subscriptions.
			query.Bool(
				query.Should(BuildSubscriptionQueries(user, request.View, subscriptions)...),
			),
			// Must be published/updated since the given time.
			query.Bool(
				query.Should(
					query.Since("published", since),
					query.Since("updated", since),
				),
			),
		),
		// Boost documents closer to the current time.
		query.Should(
			query.Distance("published", pivot, "now"),
			query.Distance("updated", pivot, "now"),
		),
		clause,
	), nil
}
