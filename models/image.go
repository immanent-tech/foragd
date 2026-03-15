// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import "strings"

// NewRemoteImage creates a new RemoteImage object.
func NewRemoteImage(url, title string) *RemoteImage {
	return &RemoteImage{
		URL:   new(url),
		Title: new(title),
	}
}

// GetURL returns the URL to the image.
func (i *RemoteImage) GetURL() string {
	if i.URL != nil {
		return *i.URL
	}
	return ""
}

// GetTitle returns the title (i.e., alt text) of the image, if any.
func (i *RemoteImage) GetTitle() string {
	if i.Title != nil {
		return *i.Title
	}
	return ""
}

func (i *RemoteImage) String() string {
	return strings.Join([]string{i.GetURL(), i.GetTitle()}, " ")
}
