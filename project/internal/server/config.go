// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	// ServerConfigEnvPrefix defines the environment variable prefix for reading
	// server configuration from the environment.
	ServerConfigEnvPrefix = "GOFEEDME_"
	// ServerConfigFile is the location of the server configuration file.
	ServerConfigFile = "server.toml"
)

var defaultCSP = []string{
	"default-src 'self' https://dev-zuc8oqf6gd86s4rw.us.auth0.com;",
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com;",
	"font-src 'self' data: https://fonts.gstatic.com;",
	"script-src 'self' https://unpkg.com 'unsafe-inline' 'unsafe-eval';",
	"connect-src 'self' ws://localhost:* https://dev-zuc8oqf6gd86s4rw.us.auth0.com;",
	"img-src 'self' https: data:;",
}

// Define default server configuration options.
var config = &Config{
	Port:         7000,
	ReadTimeout:  5 * time.Second,
	WriteTimeout: 10 * time.Second,
	Environment:  "development",
	LogLevel:     "debug",
	CSP:          defaultCSP,
}

var ErrLoadConfig = errors.New("error loading config")

// Config contains the server configuration options.
type Config struct {
	Secret       string        `toml:"app.secret"`
	Environment  string        `toml:"server.environment"`
	LogLevel     string        `toml:"server.log_level"`
	CSP          []string      `toml:"server.csp"`
	Port         int           `toml:"server.port"`
	ReadTimeout  time.Duration `toml:"server.read_timeout"`
	WriteTimeout time.Duration `toml:"server.write_timeout"`
}

var configSrc = koanf.New(".")

func loadConfig() error {
	// Load config file
	if err := configSrc.Load(file.Provider(ServerConfigFile), toml.Parser()); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Merge config with any environment variables.
	if err := configSrc.Load(env.Provider(ServerConfigEnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, ServerConfigEnvPrefix)), "_", ".", -1)
	}), nil); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Unmarshal config, overwriting defaults.
	if err := configSrc.Unmarshal("server", config); err != nil {
		return fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	// If no secret is set, create a new secret.
	if config.Secret == "" {
		secret, err := randomBase16String(32)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrLoadConfig, err)
		}

		config.Secret = secret
	}

	return nil
}

func randomBase16String(length int) (string, error) {
	buf := make([]byte, int(math.Ceil(float64(length)/2)))
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("unable to read from random source: %w", err)
	}

	str := hex.EncodeToString(buf)

	return str[:length], nil // strip 1 extra character we get from odd length results
}
