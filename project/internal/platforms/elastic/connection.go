// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package elastic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi"

	"github.com/joshuar/go-feed-me/internal/config"
	"github.com/joshuar/go-feed-me/internal/logging"
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

var (
	ErrConnectFailed = errors.New("elasticsearch connection failed")
	ErrSetupFailed   = errors.New("elasticsearch setup failed")
)

type Client struct {
	conn       *elasticsearch.TypedClient
	API        *typedapi.API
	Logger     *slog.Logger
	bulkStream chan []BulkOperation
}

func Connect(ctx context.Context, environment string) (*Client, error) {
	// Retrieve a logger from the context.
	logger := logging.FromContext(ctx).WithGroup("elastic")

	clientConfig, err := loadConfigOnce(logger, config.Environment())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	esclient, err := elasticsearch.NewTypedClient(*clientConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	client := &Client{API: typedapi.New(esclient), conn: esclient, Logger: logger}

	// if err := client.Setup(ctx); err != nil {
	// 	return nil, fmt.Errorf("%w: %w", ErrSetupFailed, err)
	// }

	client.bulkStream = make(chan []BulkOperation)
	go func() {
		defer close(client.bulkStream)
		client.bulkStreamWorker(ctx)
	}()

	return client, nil
}
