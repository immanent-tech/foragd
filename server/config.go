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

	defaultCompressionLevel = 5
)

var compressMimetypes = []string{"text/html", "text/css", "text/javascript", "font/woff2", "image/svg+xml"}

var cfg Config

type timeout string

func (t timeout) Validate() error {
	if _, err := time.ParseDuration(string(t)); err != nil {
		return fmt.Errorf("parse timeout: %w", err)
	}
	return nil
}

func (t timeout) Duration() time.Duration {
	duration, err := time.ParseDuration(string(t))
	if err != nil {
		return time.Minute
	}
	return duration
}

// Config contains the server configuration options.
type Config struct {
	Port         uint64  `koanf:"port"         validate:"port"`
	Host         string  `koanf:"host"         validate:"hostname|fqdn|ip"`
	CertFile     string  `koanf:"crt"          validate:"omitempty,file"`
	KeyFile      string  `koanf:"key"          validate:"omitempty,file"`
	ReadTimeout  timeout `koanf:"readtimeout"  validate:"required,validateFn"`
	WriteTimeout timeout `koanf:"writetimeout" validate:"required,validateFn"`
	IdleTimeout  timeout `koanf:"idletimeout"  validate:"required,validateFn"`
	BlockSignup  bool    `koanf:"blocksignup"`
	BlockLogin   bool    `koanf:"blocklogin"`
	ImgProxy     ImgProxyConfig
}

type ImgProxyConfig struct {
	Key     string `koanf:"key"     validate:"required,base64rawurl"`
	Salt    string `koanf:"salt"    validate:"required,base64rawurl"`
	BaseURL string `koanf:"baseurl" validate:"required,url"`
}

// loadConfigOnce loads the server configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(func() error {
	var err error
	// Load server config.
	cfg, err = config.Load[Config](serverConfigEnvPrefix)
	if err != nil {
		return fmt.Errorf("load server environment: %w", err)
	}
	// Load image proxy config into server config.
	var imgProxyCfg ImgProxyConfig
	imgProxyCfg, err = config.Load[ImgProxyConfig](imgProxyConfigEnvPrefix)
	if err != nil {
		return fmt.Errorf("load image proxy environment: %w", err)
	}
	cfg.ImgProxy = imgProxyCfg

	err = validation.Validate.Struct(cfg)
	if err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
