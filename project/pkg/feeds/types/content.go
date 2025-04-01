// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import (
	"encoding/xml"
	"errors"
)

// AsContent will convert the given value, which should be a content object from a feed specification, to the abstract
// Content type. If the conversion fails, a non-nil error is returned.
func AsContent(value any) (*Content, error) {
	data, err := xml.Marshal(value)
	if err != nil {
		return nil, errors.Join(ErrInvalidCategory, err)
	}
	var content Content
	err = xml.Unmarshal(data, &content)
	if err != nil {
		return nil, errors.Join(ErrInvalidCategory, err)
	}
	return &content, nil
}
