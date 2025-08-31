// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/immanent-tech/go-feed-me/config"
)

const (
	serverConfigEnvPrefix = config.ConfigEnvPrefix + "_SERVER"
	serverConfigPrefix    = "server"
)

var defaultCSP = []string{
	"default-src 'self' https://dev-zuc8oqf6gd86s4rw.us.auth0.com;",
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com;",
	"font-src 'self' data: https://fonts.gstatic.com;",
	"script-src 'self' 'unsafe-eval' 'unsafe-inline';",
	"connect-src 'self' wss://localhost:*  https://dev-zuc8oqf6gd86s4rw.us.auth0.com;",
	"img-src 'self' https: data:;",
	"frame-ancestors 'self';",
	"form-action 'self'",
	"upgrade-insecure-requests;",
}

// Define default server configuration options.
var ServerConfig = &Config{
	Port:         7000,
	ReadTimeout:  5 * time.Second,
	WriteTimeout: 10 * time.Second,
	CSP:          defaultCSP,
}

var ErrLoadConfig = errors.New("error loading config")

// Config contains the server configuration options.
type Config struct {
	Secret       string        `toml:"app.secret"`
	CSP          []string      `toml:"server.csp"`
	Port         int           `toml:"server.port"`
	ReadTimeout  time.Duration `toml:"server.read_timeout"`
	WriteTimeout time.Duration `toml:"server.write_timeout"`
}

func randomBase16String(length int) (string, error) {
	buf := make([]byte, int(math.Ceil(float64(length)/2)))
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("unable to read from random source: %w", err)
	}

	str := hex.EncodeToString(buf)

	return str[:length], nil // strip 1 extra character we get from odd length results
}
