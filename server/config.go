// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	serverConfigEnvPrefix   = config.ConfigEnvPrefix
	imgProxyConfigEnvPrefix = config.ConfigEnvPrefix + "IMGPROXY_"
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
	CSP          []string
	Port         int           `koanf:"port"`
	Host         string        `koanf:"host"`
	CertFile     string        `koanf:"crt"`
	KeyFile      string        `koanf:"key"`
	ReadTimeout  time.Duration `koanf:"read_timeout"`
	WriteTimeout time.Duration `koanf:"write_timeout"`
	IdleTimeout  time.Duration `koanf:"idle_timeout"`
	ImgProxy     ImgProxyConfig
}

type ImgProxyConfig struct {
	Key     string `koanf:"key"     validate:"required,base64rawurl"`
	Salt    string `koanf:"salt"    validate:"required,base64rawurl"`
	BaseURL string `koanf:"baseurl" validate:"required,url"`
}

// LoadConfigOnce loads the server configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfigOnce = sync.OnceValue(func() error {
	// Load server config.
	err := config.Load(serverConfigEnvPrefix, cfg)
	if err != nil {
		return fmt.Errorf("load server environment: %w", err)
	}
	// Load image proxy config into server config.
	err = config.Load(imgProxyConfigEnvPrefix, cfg.ImgProxy)
	if err != nil {
		return fmt.Errorf("load image proxy environment: %w", err)
	}

	err = validation.Validate.Struct(cfg)
	if err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
