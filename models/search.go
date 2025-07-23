// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"strconv"

	"github.com/gohugoio/hashstructure"
	"github.com/joshuar/go-syndication/sanitization"

	"github.com/joshuar/go-feed-me/validation"
)

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
	return nil
}

// ID generates an ID (hash) from the search data.
func (r *SearchRequest) ID() string {
	hash, err := hashstructure.Hash(r, nil)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(hash, 10)
}
