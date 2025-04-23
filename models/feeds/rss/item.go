// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package rss

import (
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/models/feeds/sanitization"
	"github.com/joshuar/go-feed-me/models/feeds/types"
)

var _ types.ItemSource = (*Item)(nil)

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
	switch {
	case i.DCTitle != nil:
		return i.DCTitle.String()
	case i.Title != nil:
		return sanitization.SanitizeString(*i.Title)
	default:
		return ""
	}
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
	switch {
	case i.DCDescription != nil:
		return i.DCDescription.String()
	case i.Description != nil:
		return sanitization.SanitizeString(*i.Description)
	default:
		return ""
	}
}

// GetAuthors retrieves the authors (if any) of the Item. This will be the list of values from any <author> and
// <dc:creator> elements.
func (i *Item) GetAuthors() []string {
	var authors []string
	if i.Author != nil {
		authors = append(authors, *i.Author)
	}
	if i.DCCreator != nil {
		authors = append(authors, i.DCCreator.String())
	}
	return authors
}

// GetContributors retrieves the contributors (if any) of the Item. This will be the list of values from the
// <dc:contributor> element.
func (i *Item) GetContributors() []string {
	var contributors []string
	if i.DCContributor != nil {
		contributors = append(contributors, i.DCContributor.String())
	}
	return contributors
}

// GetRights retrieves the rights (copyright) of the Channel. This will be the value of <dc:rights>, if found.
func (i *Item) GetRights() string {
	if i.DCRights != nil {
		return i.DCRights.String()
	}
	return ""
}

// GetLanguage retrieves the language of the Item. This will be the value found from the <dc:language> element, if
// present.
func (i *Item) GetLanguage() string {
	switch {
	case i.DCLanguage != nil:
		return i.DCLanguage.String()
	default:
		return ""
	}
}

// GetCategories retrieves the categories (if any) of the Item. The categories are returned as strings.
func (i *Item) GetCategories() []string {
	categories := make([]string, 0, len(i.Categories))
	for category := range slices.Values(i.Categories) {
		categories = append(categories, category.String())
	}
	return categories
}

// GetImage retrieves the image (if any) for the Item. The image is returned as a types.Image object. The value will be
// the first found of either any <image> or <media:thumbnail> element (<media:content> elements are searched
// recursively). Any errors is retrieving the image will result in a nil result being returned.
func (i *Item) GetImage() *types.Image {
	switch {
	case i.Image != nil:
		return &types.Image{
			Value: i.Image.Link,
			Title: &i.Image.Title,
		}
	case len(i.MediaThumbnails) > 0:
		// Use the first thumbnail found.
		return i.MediaThumbnails[0].AsImage()
	case i.MediaContent != nil:
		return i.MediaContent.GetImage()
	case i.Enclosure != nil && types.IsImage(i.Enclosure.Type):
		return &types.Image{
			Value: i.Enclosure.URL,
		}
	default:
		return nil
	}
}

// GetPublishedDate returns the <pubDate> of the Item (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (i *Item) GetPublishedDate() time.Time {
	if i.PubDate != nil {
		return i.PubDate.Time
	}
	return time.Unix(0, 0)
}

// GetUpdatedDate returns the <pubDate> of the Item (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (i *Item) GetUpdatedDate() time.Time {
	return i.GetPublishedDate()
}

// GetContent returns the content of the Item (if any). This will be taken from any <content:encoded> element.
func (i *Item) GetContent() *types.Content {
	if i.ContentEncoded != nil {
		return &types.Content{
			Value: sanitization.SanitizeString(i.ContentEncoded.Value),
		}
	}
	return &types.Content{}
}
