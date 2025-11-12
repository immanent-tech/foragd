// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package github

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Auth0.
	ConfigEnvPrefix = config.ConfigEnvPrefix + "GITHUB_"
	// ConfigPrefix is the prefix in the configuration file under which Auth0 configuration is stored.
	ConfigPrefix = "github"
)

var cfg = &Config{}

// Config structure.
type Config struct {
	Key            string `toml:"private_key" validate:"required"`
	ClientID       string `toml:"client_id" validate:"required"`
	InstallationID int    `toml:"installation_id"`
}

// LoadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfigOnce = sync.OnceValue(loadConfig)

func loadConfig() error {
	err := config.Load(ConfigPrefix, ConfigEnvPrefix, cfg)
	if err != nil {
		return fmt.Errorf("github: unable to load config: %w", err)
	}
	err = validation.Validate.Struct(cfg)
	if err != nil {
		return fmt.Errorf("github: unable to validate config: %w", err)
	}
	return nil
}
