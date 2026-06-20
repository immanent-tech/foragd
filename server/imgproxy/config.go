// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package imgproxy

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	configEnvPrefix = "IMGPROXY_"
)

type Config struct {
	Key    string `koanf:"key"    validate:"required,base64rawurl"`
	Salt   string `koanf:"salt"   validate:"required,base64rawurl"`
	Prefix string `koanf:"prefix" validate:"required,url"`
}

var cfg *Config

var loadConfig = sync.OnceValue(func() error {
	// Load server config.
	if err := config.Load(configEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load environment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})

func GetKey() (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.Key, nil
}

func GetSalt() (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.Salt, nil
}
