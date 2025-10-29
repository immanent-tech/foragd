// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/immanent-tech/foragd/config"
)

const (
	serverConfigEnvPrefix = config.ConfigEnvPrefix
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

var cfg = &Config{
	// Host is the hostname to listen on.
	Host: "",
	// Port is the port to listen on.
	Port: 7000,
	CSP:  defaultCSP,
	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	// https://blog.cloudflare.com/exposing-go-on-the-internet/
	ReadTimeout:  5 * time.Second,
	WriteTimeout: 10 * time.Second,
	IdleTimeout:  120 * time.Second,
}

// Config contains the server configuration options.
type Config struct {
	Secret       string        `toml:"app_secret"`
	CSP          []string      `toml:"csp"`
	Port         int           `toml:"port"`
	Host         string        `toml:"host"`
	CertFile     string        `toml:"crt"`
	KeyFile      string        `toml:"key"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
	IdleTimeout  time.Duration `toml:"idle_timeout"`
	ImgproxyURL  string        `toml:"imgproxy_url"`
}

func randomBase16String(length int) (string, error) {
	buf := make([]byte, int(math.Ceil(float64(length)/2)))
	_, err := rand.Read(buf)
	if err != nil {
		return "", fmt.Errorf("unable to read from random source: %w", err)
	}
	str := hex.EncodeToString(buf)
	return str[:length], nil // strip 1 extra character we get from odd length results
}
