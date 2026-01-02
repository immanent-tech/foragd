// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"sync"

	"github.com/go-resty/resty/v2"

	"github.com/immanent-tech/foragd/config"
)

var loadHTTPClient = sync.OnceValue(func() *resty.Client {
	return resty.New().SetHeader("User-Agent", config.AppName+"/"+config.Version)
})
