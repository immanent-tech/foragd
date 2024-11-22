// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import "github.com/knadh/koanf/v2"

const (
	settingsPrefix = "scheduler"
)

type settings struct {
	RedisServer string `toml:"redis_server"`
	RedisPort   int    `toml:"redis_port"`
}

func getSettings(config *koanf.Koanf) settings {
	envPrefix := settingsPrefix + "." + config.String("server.environment")

	return settings{
		RedisServer: config.String(envPrefix + ".redis_server"),
		RedisPort:   config.Int(envPrefix + ".redis_port"),
	}
}
