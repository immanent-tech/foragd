// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import (
	"slices"
	"strings"
)

// String returns any title (alt tag) for the Image.
func (i *Image) String() string {
	if i.Title != nil && *i.Title != "" {
		return *i.Title
	}
	return ""
}

// URL returns the URL to the Image.
func (i *Image) URL() string {
	return i.Value
}

// IsImage will return a boolean indicating whether the given mimetype represents an image.
func IsImage(mimetype string) bool {
	return slices.ContainsFunc(MimeTypesImage, func(v string) bool {
		return strings.Contains(mimetype, v)
	})
}
