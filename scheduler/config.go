// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package scheduler

import (
	"errors"
	"time"

	"github.com/immanent-tech/foragd/config"
)

const (
	configPrefix    = "scheduler"
	configEnvPrefix = config.ConfigEnvPrefix + configPrefix
)

var cfg = &Config{
	// Host is the hostname to listen on.
	Host: "",
	// Port is the port to listen on.
	Port: 8080,
	// https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/
	// https://blog.cloudflare.com/exposing-go-on-the-internet/
	ReadTimeout:  5 * time.Second,
	WriteTimeout: 10 * time.Second,
	IdleTimeout:  120 * time.Second,
}

var ErrLoadConfig = errors.New("error loading config")

// Config contains the server configuration options.
type Config struct {
	Port         int           `toml:"port"`
	Host         string        `toml:"host"`
	ReadTimeout  time.Duration `toml:"read_timeout"`
	WriteTimeout time.Duration `toml:"write_timeout"`
	IdleTimeout  time.Duration `toml:"idle_timeout"`
}
