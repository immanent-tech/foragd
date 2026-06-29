// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
)

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*elasticsearch.TypedClient
}

var api API

// Connect sets up a connection to Elasticsearch.
var Connect = sync.OnceValue(func() error {
	if err := loadConfigOnce(); err != nil {
		return fmt.Errorf("load config environment: %w", err)
	}

	var options []elasticsearch.Option
	options = append(options, elasticsearch.WithLogger(&Logger{EnableResponseBody: false, EnableRequestBody: false}))

	switch {
	case cfg.CloudID != "":
		options = append(options,
			elasticsearch.WithCloudID(cfg.CloudID),
			elasticsearch.WithAPIKey(cfg.APIKey))
	case len(cfg.URLs) > 0:
		options = append(options, elasticsearch.WithAddresses(cfg.URLs...))
		if cfg.CAFile != "" {
			cert, err := os.ReadFile(cfg.CAFile)
			if err != nil {
				return fmt.Errorf("read CA cert: %w", err)
			}
			options = append(options, elasticsearch.WithCACert(cert))
			if cfg.Username != "" && cfg.Password != "" {
				options = append(options,
					elasticsearch.WithBasicAuth(cfg.Username, cfg.Password),
				)
			}
		}
	default:
		return errors.New("invalid config")
	}

	esclient, err := elasticsearch.NewTyped(options...)
	if err != nil {
		return fmt.Errorf("create typed client: %w", err)
	}

	api = API{TypedClient: esclient}

	slog.Info("Elasticsearch connection created.") //nolint:sloglint // we do not pass a context.

	return nil
})

// NewConnection will set up a new connection to Elasticsearch. It loads the config for the connection from the
// environment.
func NewConnection() (*API, error) {
	if err := Connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &api, nil
}
