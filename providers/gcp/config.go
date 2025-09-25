// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package gcp

import (
	"fmt"
	"sync"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix for environment variables for GCP configuration.
	ConfigEnvPrefix = config.ConfigEnvPrefix + "GCP_"
	// ConfigPrefix is the prefix in the file for GCP configuration.
	ConfigPrefix = "gcp"
)

var cfg = &Config{}

// Config structure.
type Config struct {
	ProjectName string `toml:"project_name" validate:"required"`
	ProjectID   string `toml:"project_id" validate:"required"`
	OrgID       string `toml:"org_id" validate:"required"`
	LocationID  string `toml:"location_id" validate:"required"`
}

func GetProjectID() string {
	return cfg.ProjectID
}

func GetLocationID() string {
	return cfg.LocationID
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
