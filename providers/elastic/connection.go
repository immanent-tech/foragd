// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"

	"github.com/joshuar/go-feed-me/config"
)

const (
	maxIdleConnsPerHost = 10
	connTimeout         = time.Second
)

var defaultTransportConfig = &http.Transport{
	MaxIdleConnsPerHost:   maxIdleConnsPerHost,
	ResponseHeaderTimeout: connTimeout,
	DialContext:           (&net.Dialer{Timeout: connTimeout}).DialContext,
	TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
	},
}

// Connect will connect to Elasticsearch using the config in the server configuration file or environment variables and
// return an API object that can be used to issue requests.
func Connect(ctx context.Context) (*API, error) {
	clientConfig, err := loadConfigOnce(config.Environment())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	esclient, err := elasticsearch.NewTypedClient(*clientConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	return &API{TypedClient: esclient}, nil
}

func RawConnection(ctx context.Context) (*elasticsearch.Client, error) {
	clientConfig, err := loadConfigOnce(config.Environment())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}
	es, err := elasticsearch.NewClient(*clientConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}
	return es, nil
}
