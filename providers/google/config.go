// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Auth0.
	ConfigEnvPrefix = "GOOGLE_CLOUD_"
)

var cfg Config

// Config contains the pubsub configuration options.
type Config struct {
	ProjectID string `koanf:"project" validate:"required"`
}

// LoadConfig loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfig = sync.OnceValues(func() (*Config, error) {
	var err error
	cfg, err = config.Load[Config](ConfigEnvPrefix)
	if err != nil {
		return nil, fmt.Errorf("google: unable to load config: %w", err)
	}
	err = validation.Validate.Struct(cfg)
	if err != nil {
		return nil, fmt.Errorf("google: unable to validate config: %w", err)
	}
	return &cfg, nil
})
