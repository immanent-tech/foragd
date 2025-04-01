// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package rss

import (
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

// GetID returns an "id" for the item. This will be the value of the <guid> element, if present, or an empty string if
// not present.
func (i *Item) GetID() string {
	if i.GUID != nil {
		return i.GUID.Value
	}
	return ""
}

// GetTitle retrieves the <title> (if any) of the Item.
func (i *Item) GetTitle() string {
	if i.Title != nil {
		return *i.Title
	}
	return ""
}

// GetLink retrieves the <link> (if any) of the Item.
func (i *Item) GetLink() string {
	if i.Link != nil {
		return *i.Link
	}
	return ""
}

// GetDescription retrieves the <description> (if any) of the Item.
func (i *Item) GetDescription() string {
	if i.Description != nil {
		return *i.Description
	}
	return ""
}

// GetAuthors retrieves the authors (if any) of the Item. This will be the list of values from any <author> and
// <dc:creator> elements.
func (i *Item) GetAuthors() []string {
	var authors []string
	if i.Author != nil {
		authors = append(authors, *i.Author)
	}
	if i.DCCreator != nil {
		authors = append(authors, i.DCCreator.GetValue())
	}
	return authors
}

// GetContributors retrieves the contributors (if any) of the Item. This will be the list of values from the
// <dc:contributor> element.
func (i *Item) GetContributors() []string {
	var contributors []string
	if i.DCContributor != nil {
		contributors = append(contributors, i.DCContributor.GetValue())
	}
	return contributors
}

// GetCategories retrieves the categories (if any) of the Item. The categories are returned as types.Category objects,
// which tries to encapsulate an RSS category element in a portable format across schemas. Malformed categories will be
// discarded.
//
// If you prefer not to use the types.Category object, just retrieve the categories from Item.Categories directly.
func (i *Item) GetCategories() []*types.Category {
	categories := make([]*types.Category, 0, len(i.Categories))
	for category := range slices.Values(i.Categories) {
		genericCategory, err := types.AsCategory(category)
		if err != nil {
			continue
		}
		categories = append(categories, genericCategory)
	}
	return categories
}

// GetImage retrieves the image (if any) for the Item. The image is returned as a types.Image object. The value will be
// the first found of either any <image> or <media:thumbnail> element. Any errors is retrieving the image will result in a
// nil result being returned.
func (i *Item) GetImage() *types.Image {
	var (
		image *types.Image
		err   error
	)
	switch {
	case i.Image != nil:
		image, err = types.AsImage(i.Image)
	case len(i.MediaThumbnails) > 0:
		image, err = types.AsImage(i.MediaThumbnails[0])
	default:
		return nil
	}
	if err != nil {
		return nil
	}
	return image
}

// GetPublishedDate returns the <pubDate> of the Item (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (i *Item) GetPublishedDate() types.DateTime {
	if i.PubDate != nil {
		return *i.PubDate
	}
	return types.DateTime{Time: time.Unix(0, 0)}
}

// GetContent returns the content of the Item (if any). This will be taken from any <content:encoded> element.
func (i *Item) GetContent() *types.Content {
	if i.ContentEncoded != nil {
		content, err := types.AsContent(i.ContentEncoded)
		if err != nil {
			return nil
		}
		return content
	}
	return nil
}
