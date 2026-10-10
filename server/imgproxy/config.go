// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package imgproxy

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/go-base/config"

	"github.com/immanent-tech/go-base/validation"
)

const (
	configEnvPrefix = "IMGPROXY_"
)

type Config struct {
	Key    string `koanf:"key"    validate:"required,base64rawurl"`
	Salt   string `koanf:"salt"   validate:"required,base64rawurl"`
	Prefix string `koanf:"prefix" validate:"required,url"`
}

var loadConfig = sync.OnceValues(func() (*Config, error) {
	var cfg *Config
	// Load server config.
	if err := config.Load(configEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("load environment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
})

func GetKey() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.Key, nil
}

func GetSalt() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.Salt, nil
}
