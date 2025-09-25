// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	ConfigEnvPrefix = config.ConfigEnvPrefix + "AUTH0_"
	ConfigPrefix    = "auth0"
)

var cfg = &Config{}

// Config structure.
type Config struct {
	Domain       string `toml:"domain" validate:"required"`
	ClientID     string `toml:"client_id" validate:"required"`
	ClientSecret string `toml:"client_secret" validate:"required"`
	CallbackURL  string `toml:"callback_url" validate:"required,url"`
}

// LoadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfigOnce = sync.OnceValue(loadConfig)

func loadConfig() error {
	err := config.Load(ConfigPrefix, ConfigEnvPrefix, cfg)
	if err != nil {
		return fmt.Errorf("auth0: unable to load config: %w", err)
	}
	valid, err := validation.ValidateStruct(cfg)
	if err != nil || !valid {
		return fmt.Errorf("auth0: unable to validate config: %w", err)
	}
	return nil
}
