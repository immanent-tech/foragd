// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"net/url"
)

// // ShowUnread returns whether the current page has the "show_unread" query
// // parameter set, indicating unread items should be shown.
// func (n *APIPageNavigation) ShowRead() bool {
// 	return State(n.Current.Query().Get("state")) == StateRead
// }

// func (n *APIPageNavigation) State() State {
// 	return State(n.Current.Query().Get("state"))
// }

// func (n *APIPageNavigation) GenerateActionURL(action any) url.URL {
// 	var (
// 		actionURL *url.URL
// 		urlStr    string
// 		err       error
// 	)

// 	switch a := action.(type) {
// 	case State:
// 		urlStr, err = url.JoinPath(n.Action.Path, string(a))
// 		if err != nil {
// 			return url.URL{}
// 		}

// 		actionURL, err = url.Parse(urlStr)
// 		if err != nil {
// 			return url.URL{}
// 		}
// 	}

// 	return *actionURL
// }

// StripQueryParams will strip the given query parameters from the given URL.
func StripQueryParams(path url.URL, keys ...string) *url.URL {
	params := path.Query()
	for _, key := range keys {
		params.Del(key)
	}

	path.RawQuery = params.Encode()

	return &path
}

func SetQueryParams(path url.URL, params map[string]string) *url.URL {
	existingParams := path.Query()
	for key, value := range params {
		existingParams.Set(key, value)
	}

	path.RawQuery = existingParams.Encode()

	return &path
}
