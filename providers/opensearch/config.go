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
	opensearchConfigEnvPrefix = "OPENSEARCH_"
)

// cfg is the server config with default values.
var cfg = Config{
	DialTimeout: config.NewDuration(10 * time.Second),
	ReadTimeout: config.NewDuration(30 * time.Second),
}

type Config struct {
	User                string          `koanf:"user" validate:"required"`
	Password            string          `koanf:"password" validate:"required"`
	Hosts               []string        `koanf:"hosts" validate:"required,unique"`
	DialTimeout         config.Duration `koanf:"dialtimeout"`
	ReadTimeout         config.Duration `koanf:"readtimeout"`
	SkipTLSVerification bool            `koanf:"skiptlsverification"`
}

// loadConfigOnce loads the server configuration and ensures this is only done one time, no matter how many times it is
// called.
var loadConfigOnce = sync.OnceValue(func() error {
	// Load server configuration from environment, overwriting any defaults.
	if err := config.Load(opensearchConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load server environment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	return nil
})
