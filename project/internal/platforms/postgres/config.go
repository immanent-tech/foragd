// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package postgres

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
	// PostgresConfigEnvPrefix defines the environment variable prefix for reading
	// elastic configuration from the environment.
	PostgresConfigEnvPrefix = "GOFEEDME_POSTGRES_"
	// PostgresConfigFile is the location of the configuration file for elastic.
	PostgresConfigFile = "server.toml"
)

// Define default postgres configuration options.
var config = &Config{
	DSN: "host=postgres user=gofeedme password=gofeedme dbname=gofeedme port=5432 sslmode=disable",
}

var ErrLoadConfig = errors.New("error loading config")

// Config contains the server configuration options.
type Config struct {
	DSN string `toml:"postgres.dsn"`
}

var configSrc = koanf.New(".")

func loadConfig() error {
	// Load config file
	if err := configSrc.Load(file.Provider(PostgresConfigFile), toml.Parser()); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Merge config with any environment variables.
	configSrc.Load(env.Provider(PostgresConfigEnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, PostgresConfigEnvPrefix)), "_", ".", -1)
	}), nil)
	// Unmarshal config, overwriting defaults.
	if err := configSrc.Unmarshal("postgres", config); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	return nil
}
