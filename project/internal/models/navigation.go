// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"net/url"
)

// ShowUnread returns whether the current page has the "show_unread" query
// parameter set, indicating unread items should be shown.
func (n *APIPageNavigation) ShowUnread() bool {
	return n.Current.Query().Has("show_unread")
}

// StripQueryParams will strip the given query parameters from the given URL.
func StripQueryParams(path url.URL, keys ...string) *url.URL {
	params := path.Query()
	for _, key := range keys {
		params.Del(key)
	}

	path.RawQuery = params.Encode()

	return &path
}
