// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package config provides a global config store that other packages can utilise
// for fetching/storing configuration. The config store supports both file and
// environment configuration.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	// AppName is the application name.
	AppName = "Curately"
	// AppDescription is the catch-line of the application.
	AppDescription = "Collect, collate, craft your knowledge."
	// ConfigEnvPrefix defines the environment variable prefix for reading
	// server configuration from the environment.
	ConfigEnvPrefix = "GOFEEDME_"
	// ConfigPrefix defines the prefix used in the configuration file to find
	// global (app) config.
	ConfigPrefix = "app"
	// ConfigFile is the location of the server configuration file.
	ConfigFile = "server.toml"
)

// Config contains the global (app) configuration options.
type Config struct {
	Secret      string `toml:"app.secret"`
	Environment string `toml:"app.environment"`
	LogLevel    string `toml:"app.log_level"`
}

var (
	ErrLoadConfig    = errors.New("error loading config")
	ErrInvalidConfig = errors.New("invalid config")
)

// Version is the application/stack version.
var Version = "_UNKNOWN_"

var configSrc = koanf.New(".")

var appConfig = &Config{
	Environment: "development",
	LogLevel:    "debug",
}

// Init initializes the config store. This will load the global (app) config
// values and set up a config backend that other components can use via the Load
// method. This only happens once.
var Init = sync.OnceValue(func() error {
	if Version == "_UNKNOWN_" {
		return fmt.Errorf("%w: version not set correctly", ErrLoadConfig)
	}
	// // Read the version from the release-please manifest.
	// data, err := os.ReadFile("./.release-please-manifest.json")
	// if err != nil {
	// 	return fmt.Errorf("%w: unable to open release manifest: %w", ErrLoadConfig, err)
	// }
	// var versionInfo map[string]string
	// err = json.Unmarshal(data, &versionInfo)
	// if err != nil {
	// 	return fmt.Errorf("%w: unable to parse release manifest: %w", ErrLoadConfig, err)
	// }
	// if v, found := versionInfo["."]; !found {
	// 	return fmt.Errorf("%w: unable to find version in release manifest", ErrLoadConfig)
	// } else {
	// 	Version = v
	// }
	// Load config file
	err := configSrc.Load(file.Provider(ConfigFile), toml.Parser())
	if err != nil {
		slog.Warn("No config file found.",
			slog.Any("error", err),
		)
	}
	// Merge config with any environment variables.
	err = configSrc.Load(env.Provider(ConfigEnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, ConfigEnvPrefix)), "_", ".", 1)
	}), nil)
	if err != nil {
		slog.Warn("No environment variables loaded.",
			slog.Any("error", err),
		)
	}
	// Unmarshal config, overwriting defaults.
	err = configSrc.UnmarshalWithConf("app", appConfig, koanf.UnmarshalConf{Tag: "toml"})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	slog.Debug("Config backend initialized.")

	return nil
})

// Load will load the config for a component, using the given file and
// environment prefixes, and marshaling the config into the given config object.
// Components should take care to ensure this is called only once, where
// required.
func Load(configPrefix, envPrefix string, cfg any) error {
	// Load config file
	err := configSrc.Load(file.Provider(ConfigFile), toml.Parser())
	if err != nil {
		slog.Warn("No config file found.",
			slog.Any("error", err),
		)
	}
	// Merge config with any environment variables.
	err = configSrc.Load(env.Provider(envPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, envPrefix)), "_", ".", 1)
	}), nil)
	if err != nil {
		slog.Warn("No environment variables loaded.",
			slog.Any("error", err),
		)
	}
	// Unmarshal config, overwriting defaults.
	err = configSrc.UnmarshalWithConf(configPrefix, cfg, koanf.UnmarshalConf{Tag: "toml"})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	slog.Debug("Loading config for component.",
		slog.String("component", configPrefix))

	return nil
}

// Environment returns the app environment.
func Environment() string {
	return appConfig.Environment
}

// LogLevel returns the app logging level.
func LogLevel() string {
	return appConfig.LogLevel
}
