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

// Connect will connect to Elasticsearch using the config in the server configuration file or environment variables and
// return an API object that can be used to issue requests.
func Connect(ctx context.Context, env string) (*API, error) {
	clientConfig, err := loadConfigOnce(env)
	if err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}

	esclient, err := elasticsearch.NewTypedClient(*clientConfig)
	if err != nil {
		return nil, fmt.Errorf("could not generate client: %w", err)
	}

	return &API{TypedClient: esclient}, nil
}

func RawConnection(ctx context.Context, env string) (*elasticsearch.Client, error) {
	clientConfig, err := loadConfigOnce(env)
	if err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}
	es, err := elasticsearch.NewClient(*clientConfig)
	if err != nil {
		return nil, fmt.Errorf("could not generate client: %w", err)
	}
	return es, nil
}
