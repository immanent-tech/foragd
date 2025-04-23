// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package mrss contains objects and methods defining the MediaRSS extension.
package mrss

import "github.com/joshuar/go-feed-me/models/feeds/types"

// AsImage returns the <media:thumbnail> object as a types.Image object.
func (t *MediaThumbnail) AsImage() *types.Image {
	return &types.Image{
		Value: t.URL,
	}
}

// GetImage extracts the first <media:thumbnail> object in the <media:content> as a types.Image object.
func (t *MediaContent) GetImage() *types.Image {
	if t.Type != nil && types.IsImage(*t.Type) {
		if len(t.Thumbnails) > 0 {
			return t.Thumbnails[0].AsImage()
		}
	}
	return nil
}
