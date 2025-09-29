// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"fmt"
	"os"
	"sync"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"

	"github.com/immanent-tech/foragd/validation"

	"github.com/immanent-tech/foragd/config"
)

const (
	elasticConfigEnvPrefix = config.ConfigEnvPrefix + "ELASTIC_"
	elasticConfigPrefix    = "elastic"
)

// Define default server configuration options.
var cfg = &Config{
	Development: ConfigDevelopment{
		URLs: []string{"http://localhost:9200"},
	},
	Production: ConfigProduction{},
}

// Config is the elastic configuration. It varies depending on the environment.
type Config struct {
	Development ConfigDevelopment
	Production  ConfigProduction
}

// ConfigDevelopment are the config options for a development environment.
type ConfigDevelopment struct {
	CAFile   string   `toml:"ca_file"`
	Username string   `toml:"username" validate:"required"`
	Password string   `toml:"password" validate:"required"`
	URLs     []string `toml:"urls" validate:"required"`
}

// ConfigProduction are the config options for a production environment.
type ConfigProduction struct {
	CloudID string `toml:"cloud_id" validate:"required"`
	APIKey  string `toml:"api_key" validate:"required"`
}

// loadConfigOnce loads the elasticsearch configuration and ensures this is done
// one-time only, no matter how many times it is called.
func loadConfigOnce(environment string) (*elasticsearch.Config, error) {
	return sync.OnceValues(func() (*elasticsearch.Config, error) {
		var err error
		switch environment {
		case "development":
			c := &ConfigDevelopment{}
			err = config.Load(elasticConfigPrefix, elasticConfigEnvPrefix, c)
			if err != nil {
				return nil, fmt.Errorf("elastic: unable to load %s config: %w", environment, err)
			}
			cfg.Development = *c
		case "production":
			c := &ConfigProduction{}
			err = config.Load(elasticConfigPrefix, elasticConfigEnvPrefix, c)
			if err != nil {
				return nil, fmt.Errorf("elastic: unable to load %s config: %w", environment, err)
			}
			cfg.Production = *c
		}
		clientConfig, err := genConfig(environment)
		if err != nil {
			return nil, fmt.Errorf("elastic: unable to load %s config: %w", environment, err)
		}
		var valid bool
		switch environment {
		case "development":
			valid, err = validation.ValidateStruct(cfg.Development)
		case "production":
			valid, err = validation.ValidateStruct(cfg.Production)
		}
		if err != nil || !valid {
			return nil, fmt.Errorf("elastic: unable to load %s config: %w", environment, err)
		}

		return clientConfig, nil
	},
	)()
}

// genConfig will generate an Elasticsearch client config, required by the
// underlying package for connecting to an Elasticsearch cluster.
func genConfig(environment string) (*elasticsearch.Config, error) {
	var generated *elasticsearch.Config

	switch environment {
	case "development":
		generated = &elasticsearch.Config{
			Addresses: cfg.Development.URLs,
			Logger:    &Logger{EnableResponseBody: false, EnableRequestBody: false},
			// Logger:    &Logger{EnableResponseBody: true, EnableRequestBody: true},
			Username:  cfg.Development.Username,
			Password:  cfg.Development.Password,
			Transport: defaultTransportConfig,
		}
		if cfg.Development.CAFile != "" {
			caFileData, err := os.ReadFile(cfg.Development.CAFile)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve CA certificate file: %w", err)
			}
			generated.CACert = caFileData
		}
	case "production":
		generated = &elasticsearch.Config{
			Logger:    &Logger{EnableResponseBody: false, EnableRequestBody: false},
			CloudID:   cfg.Production.CloudID,
			APIKey:    cfg.Production.APIKey,
			Transport: defaultTransportConfig,
		}
	default:
		return nil, fmt.Errorf("%w: could not determine environment to apply config", config.ErrInvalidConfig)
	}

	return generated, nil
}
