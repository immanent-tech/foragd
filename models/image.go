// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

// NewRemoteImage creates a new RemoteImage object.
func NewRemoteImage(url, title string) *RemoteImage {
	return &RemoteImage{
		URL:   url,
		Title: new(title),
	}
}
