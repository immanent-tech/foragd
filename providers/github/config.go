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
)

var cfg = &Config{}

// Config structure.
type Config struct {
	Key            string `koanf:"privatekey"     validate:"required"`
	ClientID       string `koanf:"clientid"       validate:"required"`
	InstallationID int    `koanf:"installationid"`
}

// LoadConfigOnce loads the Auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfigOnce = sync.OnceValue(func() error {
	err := config.Load(ConfigEnvPrefix, cfg)
	if err != nil {
		return fmt.Errorf("github: unable to load config: %w", err)
	}
	err = validation.Validate.Struct(cfg)
	if err != nil {
		return fmt.Errorf("github: unable to validate config: %w", err)
	}
	return nil
})
