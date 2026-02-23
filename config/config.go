// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package config provides a global config store that other packages can utilise
// for fetching/storing configuration. The config store supports both file and
// environment configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
)

const (
	// AppName is the application name.
	AppName = "Foragd"
	// AppID is the application name formatted for use as an ID.
	AppID = "app.foragd"
	// AppDescription is the catch-line of the application.
	AppDescription = "A beautiful, web based, online feed reader. Keep your RSS, Atom and other syndication sources in one place. Stay up to date with news, blogs and other online sources, across your mobile, tablet, desktop and laptop."
	// ConfigEnvPrefix defines the environment variable prefix for reading
	// server configuration from the environment.
	ConfigEnvPrefix = "FORAGD_"
)

const (
	EnvDevelopment Environment = "development"
	EnvProduction  Environment = "production"
)

type Environment string

var (
	ErrLoadConfig    = errors.New("error loading config")
	ErrInvalidConfig = errors.New("invalid config")
)

// Version is the application/stack version.
var Version = "_UNKNOWN_"

// CurrentEnvironment is the environment in which the app is running (i.e., production, development).
var CurrentEnvironment Environment

// Init ensures the application will have appropriate Version and Envrionment vars set.
var Init = sync.OnceValue(func() error {
	// Set the version. This *must* be set to a valid value.
	if Version == "_UNKNOWN_" {
		return fmt.Errorf("%w: version not set correctly", ErrLoadConfig)
	}

	// Set the environment.
	CurrentEnvironment = Environment(os.Getenv("FORAGD_ENVIRONMENT"))

	return nil
})

// Load will load a config via environment variables with the given prefix into an object of the given type.
func Load[T any](envPrefix string) (T, error) {
	var cfg T
	// Initialise the config  object.
	configSrc := koanf.New(".")
	// Load environment variables.
	err := configSrc.Load(env.Provider(".", env.Opt{
		Prefix: envPrefix,
		TransformFunc: func(key, value string) (string, any) {
			// Transform the key.
			key = strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(key, ConfigEnvPrefix)), "_", ".")
			// Transform the value into slices, if they contain spaces.
			// Eg: MYVAR_TAGS="foo bar baz" -> tags: ["foo", "bar", "baz"]
			// This is to demonstrate that string values can be transformed to any type
			// where necessary.
			if strings.Contains(value, " ") {
				return key, strings.Split(value, " ")
			}
			return key, value
		},
	}), nil)
	if err != nil {
		return cfg, fmt.Errorf("unable to load config: %w", err)
	}
	// Unmarshal config, overwriting defaults.
	err = configSrc.Unmarshal(
		strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(envPrefix, ConfigEnvPrefix), "_")),
		&cfg,
	)
	if err != nil {
		return cfg, fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	return cfg, nil
}
