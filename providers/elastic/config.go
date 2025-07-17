// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"fmt"
	"os"
	"sync"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"

	"github.com/joshuar/go-feed-me/config"
)

const (
	elasticConfigEnvPrefix = config.ConfigEnvPrefix + "_ELASTIC_"
	elasticConfigPrefix    = "elastic"
)

var developmentConfig = &DevelopmentConfig{
	CAFile:   "../deployments/certs/ca/ca.crt",
	URLs:     []string{"https://es01:9200"},
	Username: "elastic",
	Password: "gofeedme",
}

// Define default server configuration options.
var elasticConfig = &Config{
	Development: developmentConfig,
}

// DevelopmentConfig are the configuration options for elastic in production
// deployments.
type DevelopmentConfig struct {
	CAFile   string   `toml:"ca_file"`
	Username string   `toml:"username"`
	Password string   `toml:"password"`
	URLs     []string `toml:"urls"`
}

// ProductionConfig are the configuration options for elastic in production
// deployments.
type ProductionConfig struct {
	CloudID string `toml:"cloud_id"`
	APIKey  string `toml:"api_key"`
}

// Config contains the server configuration options.
type Config struct {
	Development *DevelopmentConfig
	Production  *ProductionConfig
}

// loadConfigOnce loads the elasticsearch configuration and ensures this is done
// one-time only, no matter how many times it is called.
func loadConfigOnce(ctx context.Context, environment string) (*elasticsearch.Config, error) {
	return sync.OnceValues(func() (*elasticsearch.Config, error) {
		err := config.Load(elasticConfigPrefix, elasticConfigEnvPrefix, elasticConfig)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
		}

		clientConfig, err := genConfig(ctx, environment)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", config.ErrInvalidConfig, err)
		}

		return clientConfig, nil
	},
	)()
}

// genConfig will generate an Elasticsearch client config, required by the
// underlying package for connecting to an Elasticsearch cluster.
func genConfig(ctx context.Context, environment string) (*elasticsearch.Config, error) {
	var generated *elasticsearch.Config

	switch environment {
	case "development":
		caFileData, err := os.ReadFile(elasticConfig.Development.CAFile)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve CA certificate file: %w", err)
		}

		generated = &elasticsearch.Config{
			Addresses: elasticConfig.Development.URLs,
			Logger:    &Logger{EnableResponseBody: false, EnableRequestBody: false},
			// Logger:    &Logger{EnableResponseBody: true, EnableRequestBody: true},
			Username:  elasticConfig.Development.Username,
			Password:  elasticConfig.Development.Password,
			CACert:    caFileData,
			Transport: defaultTransportConfig,
		}
	case "production":
		generated = &elasticsearch.Config{
			Logger:    &Logger{EnableResponseBody: false, EnableRequestBody: false},
			CloudID:   elasticConfig.Production.CloudID,
			APIKey:    elasticConfig.Production.APIKey,
			Transport: defaultTransportConfig,
		}
	default:
		return nil, config.ErrInvalidConfig
	}

	return generated, nil
}
