// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"

	"github.com/immanent-tech/foragd/validation"

	"github.com/immanent-tech/foragd/config"
)

const (
	elasticConfigEnvPrefix = config.ConfigEnvPrefix + "ELASTIC_"
)

var defaultTransportConfig = &http.Transport{
	ResponseHeaderTimeout: 5 * time.Second,
	IdleConnTimeout:       120 * time.Second,
	MaxIdleConnsPerHost:   10,
	DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	TLSClientConfig: &tls.Config{
		// Only use curves which have assembly implementations
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519, // Go 1.8 only
		},
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, // Go 1.8 only
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,   // Go 1.8 only
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

			// Best disabled, as they don't provide Forward Secrecy,
			// but might be necessary for some clients
			// tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			// tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		},
	},
}

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
var loadConfigOnce = sync.OnceValues(func() (*elasticsearch.Config, error) {
	var err error
	switch config.CurrentEnvironment {
	case config.EnvDevelopment:
		var c ConfigDevelopment
		c, err = config.Load[ConfigDevelopment](elasticConfigEnvPrefix)
		if err != nil {
			return nil, fmt.Errorf("unable to load development config: %w", err)
		}
		cfg.Development = c
	case config.EnvProduction:
		var c ConfigProduction
		c, err = config.Load[ConfigProduction](elasticConfigEnvPrefix)
		if err != nil {
			return nil, fmt.Errorf("unable to load production config: %w", err)
		}
		cfg.Production = c
	}
	clientConfig, err := genConfig(config.CurrentEnvironment)
	if err != nil {
		return nil, fmt.Errorf("unable to generate config: %w", err)
	}
	switch config.CurrentEnvironment {
	case config.EnvDevelopment:
		err = validation.Validate.Struct(cfg.Development)
	case config.EnvProduction:
		err = validation.Validate.Struct(cfg.Production)
	}
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return clientConfig, nil
})

// genConfig will generate an Elasticsearch client config, required by the
// underlying package for connecting to an Elasticsearch cluster.
func genConfig(environment config.Environment) (*elasticsearch.Config, error) {
	var generated *elasticsearch.Config

	switch environment {
	case config.EnvDevelopment:
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
	case config.EnvProduction:
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
