// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/gohugoio/hashstructure"
	"github.com/immanent-tech/go-syndication/sanitization"

	"github.com/immanent-tech/foragd/validation"
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
	r.AuthorsExclude = sanitization.SanitizeString(r.AuthorsExclude)
	r.AuthorsInclude = sanitization.SanitizeString(r.AuthorsInclude)
	r.CategoriesExclude = sanitization.SanitizeString(r.CategoriesExclude)
	r.CategoriesInclude = sanitization.SanitizeString(r.CategoriesInclude)
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

// Query returns a string that represents the search as query parameters.
func (r *SearchRequest) Query() string {
	params := make(url.Values)
	params.Set("text", r.Text)
	if r.AuthorsExclude != "" {
		params.Set("authors_exclude", r.AuthorsExclude)
	}
	if r.AuthorsInclude != "" {
		params.Set("authors_include", r.AuthorsInclude)
	}
	if r.CategoriesExclude != "" {
		params.Set("categories_exclude", r.CategoriesExclude)
	}
	if r.CategoriesInclude != "" {
		params.Set("categories_include", r.CategoriesInclude)
	}
	params.Set("published_within", string(r.PublishedWithin))
	return params.Encode()
}
