// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"net/url"
)

func (n *APIPageNavigation) ShowUnread() bool {
	return n.Current.Query().Has("show_unread")
}

func StripQueryParams(path url.URL, keys ...string) *url.URL {
	params := path.Query()
	for _, key := range keys {
		params.Del(key)
	}

	path.RawQuery = params.Encode()

	return &path
}
