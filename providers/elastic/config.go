// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"os"
	"sync"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/validation"

	"github.com/immanent-tech/foragd/config"
)

const (
	elasticConfigEnvPrefix = config.ConfigEnvPrefix + "ELASTIC_"
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
	CAFile   string   `koanf:"cafile"   validate:"required,file"`
	Username string   `koanf:"username" validate:"required"`
	Password string   `koanf:"password" validate:"required"`
	URLs     []string `koanf:"urls"     validate:"required,dive,url"`
}

// ConfigProduction are the config options for a production environment.
type ConfigProduction struct {
	CloudID string `koanf:"cloudid" validate:"required"`
	APIKey  string `koanf:"apikey"  validate:"required"`
}

// loadConfigOnce loads the elasticsearch configuration and ensures this is done
// one-time only, no matter how many times it is called.
func loadConfigOnce(ctx context.Context) (*elasticsearch.Config, error) {
	return sync.OnceValues(func() (*elasticsearch.Config, error) {
		var err error
		switch config.Environment {
		case "development":
			var c ConfigDevelopment
			c, err = config.Load[ConfigDevelopment](elasticConfigEnvPrefix)
			if err != nil {
				return nil, fmt.Errorf("unable to load development config: %w", err)
			}
			cfg.Development = c
		case "production":
			var c ConfigProduction
			c, err = config.Load[ConfigProduction](elasticConfigEnvPrefix)
			if err != nil {
				return nil, fmt.Errorf("unable to load production config: %w", err)
			}
			cfg.Production = c
		}
		clientConfig, err := genConfig(config.Environment)
		if err != nil {
			return nil, fmt.Errorf("unable to generate config: %w", err)
		}
		switch config.Environment {
		case "development":
			err = validation.Validate.Struct(cfg.Development)
		case "production":
			err = validation.Validate.Struct(cfg.Production)
		}
		if err != nil {
			return nil, fmt.Errorf("config validation failed: %w", err)
		}

		slogctx.FromCtx(ctx).Debug("Loaded elastic config.")

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
