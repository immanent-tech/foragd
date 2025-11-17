// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"time"

	"github.com/immanent-tech/foragd/config"
)

const (
	serverConfigEnvPrefix = config.ConfigEnvPrefix
	serverConfigPrefix    = "server"
)

var defaultCSP = []string{
	"default-src 'self' https://dev-zuc8oqf6gd86s4rw.us.auth0.com;",
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
	CSP          []string      `toml:"csp"`
	Port         int           `toml:"port"`
	Host         string        `toml:"host"`
	CertFile     string        `toml:"crt"`
	KeyFile      string        `toml:"key"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
	IdleTimeout  time.Duration `toml:"idle_timeout"`
	ImgproxyURL  string        `toml:"imgproxy_url"`
	ImgproxyKey  string        `toml:"imgproxy_key"`
}
