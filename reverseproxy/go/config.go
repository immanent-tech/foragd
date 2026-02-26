// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package reverseproxy

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	serverConfigEnvPrefix = config.ConfigEnvPrefix + "REVERSEPROXY_"
)

var cfg Config

// Config contains the server configuration options.
type Config struct {
	Port         uint64         `koanf:"port"         validate:"port"`
	Host         string         `koanf:"host"         validate:"hostname|fqdn|ip"`
	CertFile     string         `koanf:"crt"          validate:"omitempty,file"`
	KeyFile      string         `koanf:"key"          validate:"omitempty,file"`
	ReadTimeout  config.Timeout `koanf:"readtimeout"  validate:"required,validateFn"`
	WriteTimeout config.Timeout `koanf:"writetimeout" validate:"required,validateFn"`
	IdleTimeout  config.Timeout `koanf:"idletimeout"  validate:"required,validateFn"`
	Key          string         `koanf:"key"          validate:"required,base64rawurl"`
	Salt         string         `koanf:"salt"         validate:"required,base64rawurl"`
}

// loadConfigOnce loads the server configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValue(func() error {
	// Load server config.
	if err := config.Load(serverConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load server environment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
