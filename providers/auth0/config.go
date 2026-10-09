/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package auth0

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Auth0.
	ConfigEnvPrefix = "AUTH0_"
)

// Config structure.
type Config struct {
	Domain       string `koanf:"domain"       validate:"required"`
	MgmtDomain   string `koanf:"mgmtdomain"   validate:"required"`
	ClientID     string `koanf:"clientid"     validate:"required"`
	ClientSecret string `koanf:"clientsecret" validate:"required"`
	CallbackURL  string `koanf:"callbackurl"  validate:"required,url"`
	Audience     string `koanf:"audience"`
}

// loadConfigOnce loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var loadConfigOnce = sync.OnceValues(func() (*Config, error) {
	var cfg Config
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("auth0: unable to load config: %w", err)
	}
	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("auth0: unable to validate config: %w", err)
	}
	return &cfg, nil
})
