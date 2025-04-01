// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import (
	"encoding/xml"
	"errors"
)

// ErrInvalidCategory indicates that the category data contains an invalid or unknown structure.
var ErrInvalidCategory = errors.New("invalid category data")

type Categories []Category

// AsCategory will convert the given value, which should be a category object from a feed specification, to the abstract
// Category type. If the conversion fails, a non-nil error is returned.
func AsCategory(value any) (*Category, error) {
	data, err := xml.Marshal(value)
	if err != nil {
		return nil, errors.Join(ErrInvalidCategory, err)
	}
	var category Category
	err = xml.Unmarshal(data, &category)
	if err != nil {
		return nil, errors.Join(ErrInvalidCategory, err)
	}
	return &category, nil
}
