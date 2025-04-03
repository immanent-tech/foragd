// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package atom

import (
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

var (
	_ types.ItemSource = (*Entry)(nil)
)

// GetID returns an "id" for the Entry. This will be the value of the <id> element, if present, or an empty string if
// not present.
func (e *Entry) GetID() string {
	return e.ID.Value
}

// GetTitle retrieves the <title> of the Entry.
func (e *Entry) GetTitle() string {
	if e.DCTitle != nil {
		return e.DCTitle.String()
	}
	return e.Title.String()
}

// GetLink retrieves the <link> associated with the Entry on the source webpage. This should usually be the link element
// with rel="self". If that is not found, an empty string is returned. Additional links may be present and can be
// retrieved directly from the Links field as needed.
func (e *Entry) GetLink() string {
	for link := range slices.Values(e.Links) {
		if link.Rel != nil && *link.Rel == LinkRelAlternate {
			return link.Href
		}
	}
	return ""
}

// GetDescription retrieves the <summary> (if any) of the Entry.
func (e *Entry) GetDescription() string {
	switch {
	case e.DCDescription != nil:
		return e.DCDescription.String()
	case e.Summary != nil:
		return e.Summary.String()
	default:
		return ""
	}
}

// GetAuthors retrieves the authors (if any) of the Entry. This will be the list of values from any <author> and
// <dc:creator> elements.
func (e *Entry) GetAuthors() []string {
	var authors []string
	if len(e.Authors) > 0 {
		for author := range slices.Values(e.Authors) {
			authors = append(authors, author.String())
		}
	}
	if e.DCCreator != nil {
		authors = append(authors, e.DCCreator.String())
	}
	return authors
}

// GetContributors retrieves the contributors (if any) of the Entry. This will be the list of values from any
// <contributor> and <dc:contributor> elements.
func (e *Entry) GetContributors() []string {
	var contributors []string
	if len(e.Contributors) > 0 {
		for contributor := range slices.Values(e.Contributors) {
			contributors = append(contributors, contributor.String())
		}
	}
	if e.DCContributor != nil {
		contributors = append(contributors, e.DCCreator.String())
	}
	return contributors
}

// GetCategories retrieves the categories (if any) of the Entry. The categories are returned as types.Category objects,
// which tries to encapsulate an Atom category element in a portable format across schemas. Malformed categories will be
// discarded.
//
// If you prefer not to use the types.Category object, just retrieve the categories from Item.Categories directly.
func (e *Entry) GetCategories() []*types.Category {
	categories := make([]*types.Category, 0, len(e.Categories))
	for category := range slices.Values(e.Categories) {
		c := &types.Category{Value: category.String()}
		// If there is a scheme value, copy that across.
		if category.Scheme != nil {
			domainAttr := types.NewXMLAttr("scheme", category.Scheme.Value, "")
			c.Attributes = types.Attributes{domainAttr}
		}
		categories = append(categories, c)
	}
	return categories
}

// GetImage retrieves the image (if any) for the Entry. The image is returned as a types.Image object. The value will be
// the first found of <media:thumbnail> element.
func (e *Entry) GetImage() *types.Image {
	if len(e.MediaThumbnails) > 0 {
		thumbnail := e.MediaThumbnails[0]
		return &types.Image{
			Value: thumbnail.URL,
		}
	}
	return nil
}

// GetPublishedDate returns the <published> of the Entry (if any). If there is no publish date, it will return a
// DateTime equal to Unix epoch.
func (e *Entry) GetPublishedDate() types.DateTime {
	if e.Published != nil {
		return e.Published.Value
	}
	return types.DateTime{Time: time.Unix(0, 0)}
}

// GetUpdatedDate returns the <updated> of the Entry.
func (e *Entry) GetUpdatedDate() types.DateTime {
	return e.Updated.Value
}

// GetContent returns the content of the Entry (if any). This will be either the <content> element value or its source
// attribute.
func (e *Entry) GetContent() *types.Content {
	if e.Content != nil {
		switch {
		case e.Content.Value != nil:
			return &types.Content{
				Value: types.SanitizeString(*e.Content.Value),
			}
		case e.Content.Source != nil:
			return &types.Content{
				Value: *e.Content.Source,
			}
		}
	}
	return nil
}
