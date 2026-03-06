// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	serverConfigEnvPrefix   = config.ConfigEnvPrefix
	imgProxyConfigEnvPrefix = "IMGPROXY_"
)

// cfg is the server config with default values.
var cfg = Config{
	Port:                 7000,
	Host:                 "localhost",
	CompressionLevel:     5,
	CompressionMimetypes: []string{"text/html", "text/css", "text/javascript", "font/woff2", "image/svg+xml"},
	ReadTimeout:          "30s",
	WriteTimeout:         "120s",
	IdleTimeout:          "900s",
}

// Config contains the server configuration options.
type Config struct {
	Port                 uint64         `koanf:"port"                 validate:"required,port"`
	Host                 string         `koanf:"host"                 validate:"required,hostname|fqdn|ip"`
	CompressionLevel     int            `koanf:"compressionlevel"     validate:"number"`
	CompressionMimetypes []string       `koanf:"compressionmimetypes"`
	CertFile             string         `koanf:"crt"                  validate:"omitempty,file"`
	KeyFile              string         `koanf:"key"                  validate:"omitempty,file"`
	ReadTimeout          config.Timeout `koanf:"readtimeout"          validate:"required,validateFn"`
	WriteTimeout         config.Timeout `koanf:"writetimeout"         validate:"required,validateFn"`
	IdleTimeout          config.Timeout `koanf:"idletimeout"          validate:"required,validateFn"`
	BlockSignup          bool           `koanf:"blocksignup"`
	BlockLogin           bool           `koanf:"blocklogin"`
	ImgProxy             ImgProxyConfig
}

type ImgProxyConfig struct {
	Key    string `koanf:"key"    validate:"required,base64rawurl"`
	Salt   string `koanf:"salt"   validate:"required,base64rawurl"`
	Prefix string `koanf:"prefix" validate:"required,url"`
}

// loadConfigOnce loads the server configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(func() error {
	// Load server config.
	if err := config.Load(serverConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load server environment: %w", err)
	}
	// Load additional environment variables.
	if os.Getenv("PORT") != "" {
		if port, err := strconv.ParseUint(os.Getenv("PORT"), 10, 64); err != nil {
			return fmt.Errorf("load port: %w", err)
		} else {
			cfg.Port = port
		}
	}
	// Load image proxy config into server config.
	var imgProxyCfg ImgProxyConfig
	if err := config.Load(imgProxyConfigEnvPrefix, &imgProxyCfg); err != nil {
		return fmt.Errorf("load image proxy environment: %w", err)
	}
	cfg.ImgProxy = imgProxyCfg

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
