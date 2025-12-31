// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"fmt"
	"log/slog"
	"sync"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"

	"github.com/immanent-tech/foragd/config"
)

// API is an object that provides access to the Elasticsearch API.
type API struct {
	*elasticsearch.TypedClient
}

var api API

var Connect = sync.OnceValue(func() error {
	if err := config.Init(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}
	clientConfig, err := loadConfigOnce()
	if err != nil {
		return fmt.Errorf("load config environment: %w", err)
	}

	esclient, err := elasticsearch.NewTypedClient(*clientConfig)
	if err != nil {
		return fmt.Errorf("create typed client: %w", err)
	}

	api = API{TypedClient: esclient}

	slog.Info("Elasticsearch connection created.")

	return nil
})

// NewConnection will set up a new connection to Elasticsearch. It loads the config for the connection from the
// environment.
func NewConnection() (*API, error) {
	if err := config.Init(); err != nil {
		return nil, fmt.Errorf("init config: %w", err)
	}
	clientConfig, err := loadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("load config environment: %w", err)
	}

	esclient, err := elasticsearch.NewTypedClient(*clientConfig)
	if err != nil {
		return nil, fmt.Errorf("create typed client: %w", err)
	}

	return &API{TypedClient: esclient}, nil
}
