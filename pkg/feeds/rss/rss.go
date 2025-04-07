// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package rss

import (
	"github.com/joshuar/go-feed-me/pkg/feeds/sanitization"
	"github.com/joshuar/go-feed-me/pkg/feeds/types"
)

var _ types.FeedSource = (*RSS)(nil)

// String returns the value of the Category.
func (c *Category) String() string {
	return sanitization.SanitizeString(c.Value)
}

func (r *RSS) GetTitle() string {
	return r.Channel.GetTitle()
}

func (r *RSS) GetDescription() string {
	return r.Channel.GetDescription()
}

func (r *RSS) GetSourceURL() string {
	return r.Channel.GetSourceURL()
}

func (r *RSS) SetSourceURL(url string) {
	r.Channel.SetSourceURL(url)
}

func (r *RSS) GetLink() string {
	return r.Channel.GetLink()
}

func (r *RSS) GetUpdatedDate() types.DateTime {
	return r.Channel.GetUpdatedDate()
}

func (r *RSS) GetPublishedDate() types.DateTime {
	return r.Channel.GetPublishedDate()
}

func (r *RSS) GetCategories() []types.Category {
	return r.Channel.GetCategories()
}

func (r *RSS) GetAuthors() []string {
	return r.Channel.GetAuthors()
}

func (r *RSS) GetContributors() []string {
	return r.Channel.GetContributors()
}

func (r *RSS) GetRights() string {
	return r.Channel.GetRights()
}

func (r *RSS) GetLanguage() string {
	return r.Channel.GetLanguage()
}

func (r *RSS) GetImage() *types.Image {
	return r.Channel.GetImage()
}

func (r *RSS) GetItems() []types.ItemSource {
	return r.Channel.GetItems()
}
