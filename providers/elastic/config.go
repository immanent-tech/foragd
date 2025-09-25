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
var elasticConfig = &Config{
	CAFile: "ca.crt",
	URLs:   []string{"https://es01:9200"},
}

// Config contains the server configuration options.
type Config struct {
	CAFile   string   `toml:"ca_file" validate:"required"`
	Username string   `toml:"username" validate:"required"`
	Password string   `toml:"password" validate:"required"`
	URLs     []string `toml:"urls" validate:"required,unique"`
	CloudID  string   `toml:"cloud_id"`
	APIKey   string   `toml:"api_key"`
}

// loadConfigOnce loads the elasticsearch configuration and ensures this is done
// one-time only, no matter how many times it is called.
func loadConfigOnce(environment string) (*elasticsearch.Config, error) {
	return sync.OnceValues(func() (*elasticsearch.Config, error) {
		err := config.Load(elasticConfigPrefix, elasticConfigEnvPrefix, elasticConfig)
		if err != nil {
			return nil, fmt.Errorf("elastic: unable to load config: %w", err)
		}
		clientConfig, err := genConfig(environment)
		if err != nil {
			return nil, fmt.Errorf("elastic: unable to load config: %w", err)
		}
		valid, err := validation.ValidateStruct(elasticConfig)
		if err != nil || !valid {
			return nil, fmt.Errorf("elastic: unable to validate config: %w", err)
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
		caFileData, err := os.ReadFile(elasticConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve CA certificate file: %w", err)
		}

		generated = &elasticsearch.Config{
			Addresses: elasticConfig.URLs,
			Logger:    &Logger{EnableResponseBody: false, EnableRequestBody: false},
			// Logger:    &Logger{EnableResponseBody: true, EnableRequestBody: true},
			Username:  elasticConfig.Username,
			Password:  elasticConfig.Password,
			CACert:    caFileData,
			Transport: defaultTransportConfig,
		}
	case "production":
		generated = &elasticsearch.Config{
			Logger:    &Logger{EnableResponseBody: false, EnableRequestBody: false},
			CloudID:   elasticConfig.CloudID,
			APIKey:    elasticConfig.APIKey,
			Transport: defaultTransportConfig,
		}
	default:
		return nil, config.ErrInvalidConfig
	}

	return generated, nil
}
