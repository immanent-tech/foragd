// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/immanent-tech/go-syndication/sanitization"

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
