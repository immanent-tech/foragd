// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package config provides a global config store that other packages can utilise
// for fetching/storing configuration. The config store supports both file and
// environment configuration.
package config

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"
)

const (
	// AppName is the application name.
	AppName = "Foragd"
	// AppID is the application name formatted for use as an ID.
	AppID = "foragd-app"
	// AppDescription is the catch-line of the application.
	AppDescription = "Gather what's important to you"
	// ConfigEnvPrefix defines the environment variable prefix for reading
	// server configuration from the environment.
	ConfigEnvPrefix = "FORAGD_"
)

var (
	ErrLoadConfig    = errors.New("error loading config")
	ErrInvalidConfig = errors.New("invalid config")
)

// Version is the application/stack version.
var Version = "_UNKNOWN_"

var configSrc *koanf.Koanf

// Init initializes the config store. This will load the global (app) config
// values and set up a config backend that other components can use via the Load
// method. This only happens once.
var Init = sync.OnceValue(func() error {
	if Version == "_UNKNOWN_" {
		return fmt.Errorf("%w: version not set correctly", ErrLoadConfig)
	}

	configSrc = koanf.New(".")

	return nil
})

// Load will populate the given cfg object with values from environment variables that match the given prefix.
func Load(envPrefix string, cfg any) error {
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
		return fmt.Errorf("unable to load config: %w", err)
	}
	// Unmarshal config, overwriting defaults.
	err = configSrc.Unmarshal(
		strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(envPrefix, ConfigEnvPrefix), "_")),
		&cfg,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	return nil
}
