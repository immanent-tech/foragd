// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gohugoio/hashstructure"
	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/validation"
)

// NewSearchRequest creates a new SearchRequest object with default values.
func NewSearchRequest() *SearchRequest {
	return &SearchRequest{
		PublishedWithin: SearchRequestPublishedWithinAllTime,
		View:            ViewUnread,
	}
}

// Valid returns a boolean indicating whether the search request data is valid.
func (r *SearchRequest) Valid() (bool, error) {
	valid, err := validation.ValidateStruct(r)
	if !valid || err != nil {
		return false, fmt.Errorf("search request is invalid: %w", err)
	}
	return true, nil
}

// Sanitise will sanitise the search request data.
func (r *SearchRequest) Sanitise() error {
	r.Text = sanitization.SanitizeString(r.Text)
	r.Authors = sanitization.SanitizeString(r.Authors)
	r.Categories = sanitization.SanitizeString(r.Categories)
	return nil
}

// ID generates an ID (hash) from the search data.
func (r *SearchRequest) ID() string {
	if reflect.ValueOf(r).IsZero() {
		return "invalid"
	}
	hash, err := hashstructure.Hash(r, nil)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(hash, 10)
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
		params.Set("categories", r.Categories)
	}
	if len(r.Subscriptions) > 0 {
		params.Set("subscriptions", strings.Join(r.Subscriptions, ","))
	}
	params.Set("view", string(r.View))
	params.Set("published_within", string(r.PublishedWithin))
	params.Set("timezone", r.Timezone)
	return params
}
