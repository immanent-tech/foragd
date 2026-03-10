// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

func (i *RemoteImage) GetURL() string {
	if i.URL != nil {
		return *i.URL
	}
	return ""
}

func (i *RemoteImage) GetTitle() string {
	if i.Title != nil {
		return *i.Title
	}
	return ""
}

func NewRemoteImage(url, title string) *RemoteImage {
	return &RemoteImage{
		URL:   new(url),
		Title: new(title),
	}
}
