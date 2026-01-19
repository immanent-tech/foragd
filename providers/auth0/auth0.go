// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"encoding/gob"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"

	"github.com/immanent-tech/foragd/config"
)

var loadHTTPClient = sync.OnceValue(func() *resty.Client {
	return resty.New().SetHeader("User-Agent", config.AppName+"/"+config.Version)
})

func init() {
	gob.Register(UserProfile{})
	gob.Register(oauth2.Token{})
	gob.Register(time.Time{})
}
