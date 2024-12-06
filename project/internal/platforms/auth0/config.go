// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package auth0

import (
	"errors"
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	Auth0ConfigEnvPrefix = "GOFEEDME_AUTH0_"
	Auth0ConfigPrefix    = "auth0"
	Auth0ConfigFile      = "server.toml"
)

// Default config values.
var config = &Config{}

var ErrLoadConfig = errors.New("error loading config")

// Config structure.
type Config struct {
	Domain       string `toml:"auth0.domain"`
	ClientID     string `toml:"auth0.client_id"`
	ClientSecret string `toml:"auth0.client_secret"`
}

var configSrc = koanf.New(".")

func loadConfig() error {
	// Load config file
	if err := configSrc.Load(file.Provider(Auth0ConfigFile), toml.Parser()); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Merge config with any environment variables.
	if err := configSrc.Load(env.Provider(Auth0ConfigEnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, Auth0ConfigEnvPrefix)), "_", ".", -1)
	}), nil); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Unmarshal config, overwriting defaults.
	if err := configSrc.Unmarshal(Auth0ConfigPrefix, config); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	return nil
}
