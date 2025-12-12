// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//nolint:goconst
package elastic

import (
	"fmt"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"

	"github.com/immanent-tech/foragd/config"
)

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
