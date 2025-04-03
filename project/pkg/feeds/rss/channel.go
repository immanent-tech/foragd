// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package rss

import (
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/pkg/feeds/atom"
	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

var (
	_ types.FeedSource = (*Channel)(nil)
)

// GetTitle retrieves the <title> (if any) of the Channel.
func (c *Channel) GetTitle() string {
	switch {
	case c.DCTitle != nil:
		return c.DCTitle.String()
	default:
		return types.SanitizeString(c.Title)
	}
}

// GetDescription retrieves the <description> (if any) of the Channel.
func (c *Channel) GetDescription() string {
	switch {
	case c.DCDescription != nil:
		return c.DCDescription.String()
	default:
		return types.SanitizeString(c.Description)
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

// GetContributors retrieves the contributors (if any) of the Item. This will be the list of values from the
// <dc:contributor> element.
func (c *Channel) GetContributors() []string {
	var contributors []string
	if c.DCContributor != nil {
		contributors = append(contributors, c.DCContributor.String())
	}
	return contributors
}

// GetCategories retrieves the categories (if any) of the Item. The categories are returned as types.Category objects,
// which tries to encapsulate an RSS category element in a portable format across schemas. Malformed categories will be
// discarded.
//
// If you prefer not to use the types.Category object, just retrieve the categories from Item.Categories directly.
func (c *Channel) GetCategories() []*types.Category {
	categories := make([]*types.Category, 0, len(c.Categories))
	for category := range slices.Values(c.Categories) {
		c := &types.Category{Value: category.String()}
		// If the domain attribute has a value, copy it across.
		if category.Domain != nil {
			domainAttr := types.NewXMLAttr("domain", *category.Domain, "")
			c.Attributes = types.Attributes{domainAttr}
		}
		categories = append(categories, c)
	}
	return categories
}

// GetImage retrieves the image (if any) for the Item. The image is returned as a types.Image object. The value will be
// the first found of either any <image> or <media:thumbnail> element. Any errors is retrieving the image will result in a
// nil result being returned.
func (c *Channel) GetImage() *types.Image {
	switch {
	case c.Image != nil:
		return &types.Image{
			Value: c.Image.Link,
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

// GetPublishedDate returns the <pubDate> of the Item (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (c *Channel) GetPublishedDate() types.DateTime {
	if c.PubDate != nil {
		return *c.PubDate
	}
	return types.DateTime{Time: time.Unix(0, 0)}
}

// GetUpdatedDate returns the <pubDate> of the Item (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (c *Channel) GetUpdatedDate() types.DateTime {
	return c.GetPublishedDate()
}

func (c *Channel) GetItems() []types.Item {
	items := make([]types.Item, 0, len(c.Items))
	for item := range slices.Values(c.Items) {
		items = append(items, types.Item{ItemSource: &item})
	}
	return items
}
