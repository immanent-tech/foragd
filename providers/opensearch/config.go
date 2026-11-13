/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package opensearch

import (
	"fmt"
	"sync"
	"time"

	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"
)

const (
	configEnvPrefix = "OPENSEARCH_"
)

type Config struct {
	User                string          `koanf:"user"                validate:"required"`
	Password            string          `koanf:"password"            validate:"required"`
	Hosts               []string        `koanf:"hosts"               validate:"required,unique"`
	DialTimeout         config.Duration `koanf:"dialtimeout"`
	ReadTimeout         config.Duration `koanf:"readtimeout"`
	SkipTLSVerification bool            `koanf:"skiptlsverification"`
	Settings            Settings        `koanf:"settings"`
}

// loadConfigOnce loads the server configuration and ensures this is only done one time, no matter how many times it is
// called.
var loadConfigOnce = sync.OnceValues(func() (*Config, error) {
	var cfg = Config{
		DialTimeout: config.NewDuration(10 * time.Second),
		ReadTimeout: config.NewDuration(30 * time.Second),
	}
	// Load server configuration from environment, overwriting any defaults.
	if err := config.Load(configEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("load server environment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
})

// Settings are Opensearch cluster settings.
type Settings struct {
	Plugins struct {
		MLCommons struct {
			TrustedConnectorEndpointsRegex        []string `json:"trusted_connector_endpoints_regex" koanf:"trusted_connector_endpoints_regex"`
			TrustedConnectorPrivateEndpointsRegex []string `json:"trusted_connector_private_endpoints_regex" koanf:"trusted_connector_private_endpoints_regex"`
			Connector                             struct {
				PrivateIPEnabled bool `json:"private_ip_enabled" koanf:"private_ip_enabled"`
			} `json:"connector" koanf:"connector"`
			OnlyRunOnMLNode bool `json:"only_run_on_ml_node" koanf:"only_run_on_ml_node"`
		} `json:"ml_commons" koanf:"ml_commons"`
	} `json:"plugins" koanf:"plugins"`
}
