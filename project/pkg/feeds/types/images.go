// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import (
	"encoding/xml"
	"errors"
)

var ErrInvalidImage = errors.New("invalid image data")

// AsImage will convert the given value, which should be a image object from a feed specification, to the abstract
// Image type. If the conversion fails, a non-nil error is returned.
func AsImage(value any) (*Image, error) {
	data, err := xml.Marshal(value)
	if err != nil {
		return nil, errors.Join(ErrInvalidCategory, err)
	}
	var image Image
	err = xml.Unmarshal(data, &image)
	if err != nil {
		return nil, errors.Join(ErrInvalidCategory, err)
	}
	return &image, nil
}

// GetValue retrieves the value of the Image. It will retrieve the first of either the value field or URL attribute.
func (i *Image) GetValue() string {
	switch {
	case i.Value != nil && *i.Value != "":
		return *i.Value
	case i.URL != nil && *i.URL != "":
		return *i.URL
	default:
		return ""
	}
}
