// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"fmt"
	"os"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Auth0.
	ConfigEnvPrefix = "GOOGLE_CLOUD_"
)

var cfg = Config{
	Service:  os.Getenv("K_SERVICE"),
	Revision: os.Getenv("K_REVISION"),
}

// Config contains the pubsub configuration options.
type Config struct {
	ProjectID string `koanf:"project" validate:"required"`
	Service   string
	Revision  string
}

// LoadConfig loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfig = sync.OnceValues(func() (*Config, error) {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("google: unable to load config: %w", err)
	}
	if cfg.ProjectID == "" {
		// Try to fetch details from metadata endpoint.
		cfg.ProjectID, _ = queryMetadataServer("/computeMetadata/v1/project/project-id")
	}
	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("google: unable to validate config: %w", err)
	}
	return &cfg, nil
})
