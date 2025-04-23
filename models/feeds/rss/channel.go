// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package rss

import (
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/models/feeds/atom"
	"github.com/joshuar/go-feed-me/models/feeds/sanitization"
	"github.com/joshuar/go-feed-me/models/feeds/types"
)

var _ types.FeedSource = (*Channel)(nil)

// GetTitle retrieves the <title> (if any) of the Channel.
func (c *Channel) GetTitle() string {
	switch {
	case c.DCTitle != nil:
		return c.DCTitle.String()
	default:
		return sanitization.SanitizeString(c.Title)
	}
}

// GetDescription retrieves the <description> (if any) of the Channel.
func (c *Channel) GetDescription() string {
	switch {
	case c.DCDescription != nil:
		return c.DCDescription.String()
	default:
		return sanitization.SanitizeString(c.Description)
	}
}

// GetSourceURL retrieves the URL that links to the RSS file for the channel. This will be any <atom:link> element
// present in the Channel with a "rel" attribute of "self".
func (c *Channel) GetSourceURL() string {
	if c.AtomLink == nil {
		return ""
	}
	if c.AtomLink.Rel != nil && *c.AtomLink.Rel == atom.LinkRelSelf {
		return c.AtomLink.Href
	}
	return ""
}

// SetSourceURL will set a source URL, indicating the URL to the RSS file, in the Channel.
func (c *Channel) SetSourceURL(url string) {
	rel := atom.LinkRelSelf
	c.AtomLink = &atom.Link{Href: url, Rel: &rel}
}

// GetLink retrieves the <link> (if any) of the Channel. This is the link to the website associated with the RSS feed.
func (c *Channel) GetLink() string {
	return c.Link
}

// GetAuthors retrieves the authors (if any) of the Channel. This will be the list of values from any <dc:creator>
// elements.
func (c *Channel) GetAuthors() []string {
	var authors []string
	if c.DCCreator != nil {
		authors = append(authors, c.DCCreator.String())
	}
	return authors
}

// GetContributors retrieves the contributors (if any) of the Channel. This will be the list of values from the
// <dc:contributor> element.
func (c *Channel) GetContributors() []string {
	var contributors []string
	if c.DCContributor != nil {
		contributors = append(contributors, c.DCContributor.String())
	}
	return contributors
}

// GetRights retrieves the rights (copyright) of the Channel. This will be the first value found from either <dc:rights>
// or <copyright> elements.
func (c *Channel) GetRights() string {
	switch {
	case c.DCRights != nil:
		return c.DCRights.String()
	case c.Copyright != nil:
		return *c.Copyright
	default:
		return ""
	}
}

// GetLanguage retrieves the language of the Channel. This will be the first value found from either <dc:language>
// or <lang> elements.
func (c *Channel) GetLanguage() string {
	switch {
	case c.DCLanguage != nil:
		return c.DCLanguage.String()
	case c.Language != nil:
		return *c.Language
	default:
		return ""
	}
}

// GetCategories retrieves the categories (if any) of the Channel. The categories are returned as strings.
func (c *Channel) GetCategories() []string {
	categories := make([]string, 0, len(c.Categories))
	for category := range slices.Values(c.Categories) {
		categories = append(categories, category.String())
	}
	return categories
}

// GetImage retrieves the image (if any) for the Item. The image is returned as a types.Image object. The value will be
// the first found of either any <image> or <media:thumbnail> element. Any errors is retrieving the image will result in
// a nil result being returned.
func (c *Channel) GetImage() *types.Image {
	switch {
	case c.Image != nil:
		return &types.Image{
			Value: c.Image.URL,
			Title: &c.Image.Title,
		}
	case len(c.MediaThumbnails) > 0:
		// Use the first thumbnail found.
		thumbnail := c.MediaThumbnails[0]
		return &types.Image{
			Value: thumbnail.URL,
		}
	default:
		return nil
	}
}

// SetImage sets an image for the Channel.
func (c *Channel) SetImage(image *types.Image) {
	c.Image = &Image{URL: image.URL(), Title: image.String()}
}

// GetPublishedDate returns the <pubDate> of the Item (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (c *Channel) GetPublishedDate() time.Time {
	if c.PubDate != nil {
		return c.PubDate.Time
	}
	return time.Unix(0, 0)
}

// GetUpdatedDate returns the <pubDate> of the Item (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (c *Channel) GetUpdatedDate() time.Time {
	return c.GetPublishedDate()
}

// GetItems retrieves a slice of Item values for the Channel.
func (c *Channel) GetItems() []types.ItemSource {
	items := make([]types.ItemSource, 0, len(c.Items))
	for item := range slices.Values(c.Items) {
		items = append(items, &item)
	}
	return items
}
