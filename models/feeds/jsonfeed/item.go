// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package jsonfeed

import (
	"slices"
	"time"

	"github.com/joshuar/go-feed-me/models/feeds/sanitization"
	"github.com/joshuar/go-feed-me/models/feeds/types"
)

var _ types.ItemSource = (*Item)(nil)

// GetID returns an "id" for the Item.
func (i *Item) GetID() string {
	return i.ID
}

// GetTitle retrieves title (if any) of the Item.
func (i *Item) GetTitle() string {
	if i.Title != nil {
		return sanitization.SanitizeString(*i.Title)
	}
	return ""
}

// GetLink retrieves URL associated with the item (if any). It will retrieve either the URL or ExternalURL, whichever is
// found first, or an empty string if neither is found.
func (i *Item) GetLink() string {
	switch {
	case i.URL != nil:
		return *i.URL
	case i.ExternalURL != nil:
		return *i.ExternalURL
	default:
		return ""
	}
}

// GetDescription retrieves the summary (if any) of the Item.
func (i *Item) GetDescription() string {
	if i.Summary != nil {
		return sanitization.SanitizeString(*i.Summary)
	}
	return ""
}

// GetAuthors retrieves the authors (if any) of the Item.
func (i *Item) GetAuthors() []string {
	authors := make([]string, 0, len(i.Authors))
	if i.Author != nil {
		authors = append(authors, i.Author.String())
	}
	for author := range slices.Values(i.Authors) {
		if author.String() != "" {
			authors = append(authors, author.String())
		}
	}
	return authors
}

// GetContributors is a no-op for JSONFeed items.
func (i *Item) GetContributors() []string {
	return nil
}

// GetRights is a no-op for JSONFeed items.
func (i *Item) GetRights() string {
	return ""
}

// GetLanguage retrieves the language (if any) of the item.
func (i *Item) GetLanguage() string {
	if i.Language != nil {
		return *i.Language
	}
	return ""
}

// GetCategories retrieves the categories (if any) of the Item.
func (i *Item) GetCategories() []string {
	return i.Tags
}

// GetImage retrieves the image (if any) for the Item.
func (i *Item) GetImage() *types.Image {
	if i.Image != nil {
		return &types.Image{
			Value: *i.Image,
		}
	}
	return nil
}

// GetPublishedDate returns the published date of the Item.
func (i *Item) GetPublishedDate() time.Time {
	if i.DatePublished != nil {
		return i.DatePublished.Time
	}
	return time.Unix(0, 0)
}

// GetUpdatedDate returns the updated (modified) date of the Item.
func (i *Item) GetUpdatedDate() time.Time {
	if i.DateModified != nil {
		return i.DateModified.Time
	}
	return time.Unix(0, 0)
}

// GetContent returns the content of the Item (if any). This will be either the html or text content, whichever is found
// first.
func (i *Item) GetContent() *types.Content {
	var content string
	switch {
	case i.ContentHTML != nil:
		content = sanitization.SanitizeString(*i.ContentHTML)
	case i.ContentText != nil:
		content = sanitization.SanitizeString(*i.ContentText)
	}
	if content != "" {
		return &types.Content{
			Value: content,
		}
	}
	return nil
}
