// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

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
	SchedulerConfigEnvPrefix = "GOFEEDME_SCHEDULER_"
	SchedulerConfigPrefix    = "scheduler"
	SchedulerConfigFile      = "server.toml"
)

// Default config values.
var config = &Config{
	RedisServer: "valkey",
	RedisPort:   6379,
}

var ErrLoadConfig = errors.New("error loading config")

// Config structure.
type Config struct {
	RedisServer string `toml:"redis.server"`
	RedisPort   int    `toml:"redis.port"`
}

var configSrc = koanf.New(".")

func loadConfig() error {
	// Load config file
	if err := configSrc.Load(file.Provider(SchedulerConfigFile), toml.Parser()); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Merge config with any environment variables.
	if err := configSrc.Load(env.Provider(SchedulerConfigEnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, SchedulerConfigEnvPrefix)), "_", ".", -1)
	}), nil); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Unmarshal config, overwriting defaults.
	if err := configSrc.UnmarshalWithConf(SchedulerConfigPrefix, config, koanf.UnmarshalConf{Tag: "toml"}); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	return nil
}
