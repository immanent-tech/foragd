// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	// ElasticConfigEnvPrefix defines the environment variable prefix for reading
	// elastic configuration from the environment.
	ElasticConfigEnvPrefix = "GOFEEDME_ELASTIC_"
	ElasticConfigPrefix    = "elastic"
	// ElasticConfigFile is the location of the configuration file for elastic.
	ElasticConfigFile = "server.toml"
)

var developmentConfig = &DevelopmentConfig{
	CAFile:   "../deployments/certs/ca/ca.crt",
	URLs:     []string{"https://es01:9200"},
	Username: "elastic",
	Password: "gofeedme",
}

// Define default server configuration options.
var config = &Config{
	Development: developmentConfig,
}

var (
	ErrLoadConfig    = errors.New("error loading config")
	ErrInvalidConfig = errors.New("invalid config")
)

// DevelopmentConfig are the configuration options for elastic in production
// deployments.
type DevelopmentConfig struct {
	CAFile   string   `toml:"elastic.development.ca_file"`
	Username string   `toml:"elastic.development.username"`
	Password string   `toml:"elastic.development.password"`
	URLs     []string `toml:"elastic.development.urls"`
}

// ProductionConfig are the configuration options for elastic in production
// deployments.
type ProductionConfig struct {
	CloudID string `toml:"elastic.production.cloud_id"`
	APIKey  string `toml:"elastic.production.api_key"`
}

// Config contains the server configuration options.
type Config struct {
	Development *DevelopmentConfig
	Production  *ProductionConfig
}

var configSrc = koanf.New(".")

func loadConfig(logger *slog.Logger, environment string) (*elasticsearch.Config, error) {
	// Load config file
	if err := configSrc.Load(file.Provider(ElasticConfigFile), toml.Parser()); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Merge config with any environment variables.
	if err := configSrc.Load(env.Provider(ElasticConfigEnvPrefix, ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, ElasticConfigEnvPrefix)), "_", ".", -1)
	}), nil); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}
	// Unmarshal config, overwriting defaults.
	if err := configSrc.Unmarshal(ElasticConfigPrefix+"."+environment, config); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLoadConfig, err)
	}

	esconfig, err := genConfig(logger, environment)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	return esconfig, nil
}

func genConfig(logger *slog.Logger, environment string) (*elasticsearch.Config, error) {
	var generated *elasticsearch.Config

	switch environment {
	case "development":
		caFileData, err := os.ReadFile(config.Development.CAFile)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve CA certificate file: %w", err)
		}

		generated = &elasticsearch.Config{
			Addresses: config.Development.URLs,
			Logger: &ESLogger{
				Logger: *logger,
			},
			Username:  config.Development.Username,
			Password:  config.Development.Password,
			CACert:    caFileData,
			Transport: defaultTransportConfig,
		}
	case "production":
		generated = &elasticsearch.Config{
			Logger: &ESLogger{
				Logger: *logger,
			},
			CloudID:   config.Production.CloudID,
			APIKey:    config.Production.APIKey,
			Transport: defaultTransportConfig,
		}
	default:
		return nil, ErrInvalidConfig
	}

	return generated, nil
}
