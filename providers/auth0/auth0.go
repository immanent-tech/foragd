// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"fmt"
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
	gob.Register(map[string]string{})
}

// GenerateRandomState generates a new nonce that can be used during authentication as a state parameter.
func GenerateRandomState() (string, error) {
	const stateSize = 32
	bytes := make([]byte, stateSize)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("unable to generate random state: %w", err)
	}

	state := base64.StdEncoding.EncodeToString(bytes)

	return state, nil
}
