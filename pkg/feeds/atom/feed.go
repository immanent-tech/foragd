// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package atom

import (
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

var _ types.FeedSource = (*Feed)(nil)

// GetTitle retrieves the <title> of the Feed.
func (f *Feed) GetTitle() string {
	switch {
	case f.DCTitle != nil:
		return f.DCTitle.String()
	case f.Title.String() != "":
		return f.Title.String()
	default:
		return ""
	}
}

// GetDescription retrieves the <description> (if any) of the Feed.
func (f *Feed) GetDescription() string {
	switch {
	case f.DCDescription != nil:
		return f.DCDescription.String()
	case f.Subtitle != nil:
		return f.Subtitle.String()
	default:
		return ""
	}
}

// GetSourceURL retrieves the URL that links to the Atom file for the Feed. This will be any <link> element
// present with a "rel" attribute of "self".
func (f *Feed) GetSourceURL() string {
	for link := range slices.Values(f.Links) {
		if link.Rel != nil && *link.Rel == LinkRelSelf {
			return link.Href
		}
	}
	return ""
}

// SetSourceURL will set a source URL, indicating the URL of the Atom document, in the Feed.
func (f *Feed) SetSourceURL(url string) {
	rel := LinkRelSelf
	f.Links = append(f.Links, Link{Href: url, Rel: &rel})
}

// GetLink retrieves the <link> of the Feed. This is the link to the website associated with the RSS feed.
func (f *Feed) GetLink() string {
	for link := range slices.Values(f.Links) {
		if link.Rel != nil && *link.Rel == LinkRelAlternate {
			return link.Href
		}
	}
	return ""
}

// GetAuthors retrieves the authors (if any) of the Feed. This will be the list of values from any <author> and
// <dc:creator> elements.
func (f *Feed) GetAuthors() []string {
	var authors []string
	if len(f.Authors) > 0 {
		for author := range slices.Values(f.Authors) {
			authors = append(authors, author.String())
		}
	}
	if f.DCCreator != nil {
		authors = append(authors, f.DCCreator.String())
	}
	return authors
}

// GetContributors retrieves the contributors (if any) of the Feed. This will be the list of values from any
// <contributor> and <dc:contributor> elements.
func (f *Feed) GetContributors() []string {
	var contributors []string
	if len(f.Contributors) > 0 {
		for contributor := range slices.Values(f.Contributors) {
			contributors = append(contributors, contributor.String())
		}
	}
	if f.DCContributor != nil {
		contributors = append(contributors, f.DCCreator.String())
	}
	return contributors
}

// GetRights retrieves the rights (copyright) of the Feed. This will be the first value found from either <dc:rights>
// or <rights> elements.
func (f *Feed) GetRights() string {
	switch {
	case f.DCRights != nil:
		return f.DCRights.String()
	case f.Rights != nil:
		return f.Rights.Value
	default:
		return ""
	}
}

// GetLanguage retrieves the language of the Feed. This will be the first value found from either <dc:language>
// or <lang> elements.
func (f *Feed) GetLanguage() string {
	switch {
	case f.DCLanguage != nil:
		return f.DCLanguage.String()
	case f.Lang != nil:
		return *f.Lang
	default:
		return ""
	}
}

// GetCategories retrieves the categories (if any) of the Feed. The categories are returned as types.Category objects,
// which tries to encapsulate an Atom category element in a portable format across schemas. Malformed categories will be
// discarded.
//
// If you prefer not to use the types.Category object, just retrieve the categories from Item.Categories directly.
func (f *Feed) GetCategories() []types.Category {
	categories := make([]types.Category, 0, len(f.Categories))
	for category := range slices.Values(f.Categories) {
		c := types.Category{Value: category.String()}
		// If there is a scheme value, copy that across.
		if category.Scheme != nil {
			domainAttr := types.NewXMLAttr("scheme", category.Scheme.Value, "")
			c.Attributes = types.Attributes{domainAttr}
		}
		categories = append(categories, c)
	}
	return categories
}

// GetImage retrieves the image (if any) for the Feed. The image is returned as a types.Image object. The value will be
// the first found of <media:thumbnail> element.
func (f *Feed) GetImage() *types.Image {
	if len(f.MediaThumbnails) > 0 {
		thumbnail := f.MediaThumbnails[0]
		return &types.Image{
			Value: thumbnail.URL,
		}
	}
	return nil
}

func (f *Feed) GetPublishedDate() time.Time {
	return f.Updated.Value.Time
}

// GetUpdatedDate returns the <updated> of the Feed.
func (f *Feed) GetUpdatedDate() time.Time {
	return f.Updated.Value.Time
}

func (f *Feed) GetItems() []types.ItemSource {
	items := make([]types.ItemSource, 0, len(f.Entries))
	for item := range slices.Values(f.Entries) {
		items = append(items, &item)
	}
	return items
}
