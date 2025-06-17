// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import "errors"

var (
	ErrAPIRequestFailed = errors.New("api request failed")

	ErrConnectFailed = errors.New("elasticsearch connection failed")
	ErrSetupFailed   = errors.New("elasticsearch setup failed")

	ErrNoClient      = errors.New("no client")
	ErrFieldNotFound = errors.New("field not found")
	ErrReqFailed     = errors.New("api request failed")
	ErrNotFound      = errors.New("not found")

	ErrPagination = errors.New("pagination error")
	// Doc Errors.
	ErrUpdateFailed = errors.New("update failed")
	ErrExistsFailed = errors.New("exists request failed")

	ErrGetFailed = errors.New("get request failed")

	ErrPutILMPolicyFailed = errors.New("create ILM policy failed")

	ErrSearchFailed = errors.New("search failed")
	ErrCountFailed  = errors.New("count failed")
	ErrNoHits       = errors.New("no hits found")
)
