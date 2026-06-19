// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package config provides a global config store that other packages can utilise
// for fetching/storing configuration. The config store supports both file and
// environment configuration.
package config

import (
	"errors"
	"fmt"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/v2"

	"github.com/immanent-tech/foragd/validation"
)

const (
	// AppName is the application name.
	AppName = "Foragd"
	// AppID is the application name formatted for use as an ID.
	AppID = "app.foragd"
	// AppDescription is the catch-line of the application.
	AppDescription = "Foragd is a web-based RSS and feed reader. Follow any website, blog or newsletter in one place. No algorithms, no ads, fully searchable."
	// ConfigEnvPrefix defines the environment variable prefix for reading
	// server configuration from the environment.
	ConfigEnvPrefix = "FORAGD_"
)

const (
	// EnvDevelopment represents a development environment.
	EnvDevelopment Environment = "development"
	// EnvProduction represents a production environment.
	EnvProduction Environment = "production"
)

// Environment is the app running environment.
type Environment string

func (e Environment) String() string {
	return string(e)
}

var (
	ErrLoadConfig    = errors.New("error loading config")
	ErrInvalidConfig = errors.New("invalid config")
)

type appConfig struct {
	// Version is the application/stack version.
	Version string `koanf:"version" validate:"required,ne=_UNKNOWN_"`
	// CurrentEnvironment is the environment in which the app is running (i.e., production, development). Defaults to
	// "development".
	Environment Environment `koanf:"environment" validate:"required,oneof=production development"`
	// BaseURL is the base url from which the app is being served.
	BaseURL string `koanf:"baseurl" validate:"required,url"`
}

var cfg = &appConfig{
	Environment: EnvDevelopment,
	Version:     "_UNKNOWN_",
}

var loadConfig = sync.OnceValue(func() error {
	var vcsRevision string
	// var vcsTime string
	var vcsModified bool
	var vcsSystem string
	if info, ok := debug.ReadBuildInfo(); ok {
		for buildInfo := range slices.Values(info.Settings) {
			switch buildInfo.Key {
			case "vcs":
				vcsSystem = buildInfo.Value
			case "vcs.revision":
				vcsRevision = buildInfo.Value
			// case "vcs.time":
			// 	vcsTime = s.Value
			case "vcs.modified":
				vcsModified = buildInfo.Value == "true"
			}
		}
		cfg.Version = strings.Join([]string{vcsSystem, vcsRevision}, "-")
		if vcsModified {
			cfg.Version += "-dirty"
		}
	}

	if err := Load(ConfigEnvPrefix, cfg); err != nil {
		return fmt.Errorf("load base config: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate base config: %w", err)
	}

	return nil
})

func GetVersion() string {
	if err := loadConfig(); err != nil {
		panic(err)
	}
	return cfg.Version
}

func GetBaseURL() string {
	if err := loadConfig(); err != nil {
		panic(err)
	}
	return cfg.BaseURL
}

func GetEnvironment() Environment {
	if err := loadConfig(); err != nil {
		panic(err)
	}
	return cfg.Environment
}

func IsProduction() bool {
	if err := loadConfig(); err != nil {
		panic(err)
	}
	return cfg.Environment == EnvProduction
}

// Load will load a config via environment variables with the given prefix into an object of the given type.
func Load[T any](envPrefix string, cfg T) error {
	// Initialize the config object.
	configSrc := koanf.New(".")
	// Load environment variables.
	if err := configSrc.Load(env.Provider(".", env.Opt{
		Prefix: envPrefix,
		TransformFunc: func(key, value string) (string, any) {
			// Lowercase and remove the prefix.
			key = strings.ToLower(strings.TrimPrefix(key, envPrefix))
			// Split space-separate values into a slice.
			if strings.Contains(value, " ") {
				return key, strings.Split(value, " ")
			}
			return key, value
		},
	}), nil); err != nil {
		return fmt.Errorf("unable to load config: %w", err)
	}
	// Unmarshal config, overwriting defaults.
	if err := configSrc.Unmarshal("", cfg); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	return nil
}
