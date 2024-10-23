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
	"os"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi"
	"github.com/knadh/koanf/v2"

	"github.com/joshuar/go-feed-me/logging"
	"github.com/joshuar/go-feed-me/platform/elastic/schema"
	"github.com/joshuar/go-feed-me/platform/feeds"
)

const (
	ConfigPrefix  = "elastic"
	ConfigURL     = "url"
	ConfigCAFile  = "ca_file"
	ConfigUser    = "user"
	ConfigPass    = "pass"
	ConfigCloudID = "cloud_id"
	ConfigAPIKey  = "api_key"

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
	ErrInvalidConfig = errors.New("invalid elasticsearch config")
	ErrSetupFailed   = errors.New("elasticsearch setup failed")
)

type Client struct {
	conn                *elasticsearch.TypedClient
	API                 *typedapi.API
	logger              *slog.Logger
	feedItemsBulkStream chan []feeds.FeedItem
}

func Connect(ctx context.Context, config *koanf.Koanf) (*Client, error) {
	var (
		esconfig *elasticsearch.Config
		err      error
	)

	// Retrieve a logger from the context.
	logger := logging.FromContext(ctx).With(slog.String("platform", "elastic"))

	esconfig, err = genConfig(config, logger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	esclient, err := elasticsearch.NewTypedClient(*esconfig)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectFailed, err)
	}

	client := &Client{API: typedapi.New(esclient), conn: esclient, logger: logger}

	if err := client.Setup(ctx); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSetupFailed, err)
	}

	client.feedItemsBulkStream = make(chan []feeds.FeedItem)
	go func() {
		defer close(client.feedItemsBulkStream)
		client.bulkIndexFeedItemsWorker(ctx)
	}()

	return client, nil
}

func (c *Client) Setup(ctx context.Context) error {
	// Get the latest ILM policy for feeditems from file.
	policy, err := GetILMPolicy(schema.FeedItemsSchemaID)
	if err != nil {
		return fmt.Errorf("get ILM Policy: %w", err)
	}
	// Update the ILM policy for feeditems.
	if err := c.PutILMPolicy(ctx, schema.FeedItemsSchemaID, policy); err != nil {
		return fmt.Errorf("put ILM policy: %w", err)
	}
	// Update the feeditems index template.
	if err := c.PutIndexTemplate(ctx, schema.FeeditemsIndexTemplate()); err != nil {
		return fmt.Errorf("put index template: %w", err)
	}

	if err := c.PutIngestPipeline(ctx, schema.FeedItemsIngestPipeline()); err != nil {
		return fmt.Errorf("put ingest pipeline: %w", err)
	}

	return nil
}

func genConfig(config *koanf.Koanf, logger *slog.Logger) (*elasticsearch.Config, error) {
	var generated *elasticsearch.Config

	environment := config.String("server.environment")

	envPrefix := "elastic." + environment

	switch environment {
	case "development":
		caFileData, err := os.ReadFile(config.String(envPrefix + ".caFile"))
		if err != nil {
			return nil, fmt.Errorf("could not retrieve CA certificate file: %w", err)
		}

		generated = &elasticsearch.Config{
			Addresses: config.Strings(envPrefix + ".urls"),
			Logger: &ESLogger{
				Logger: *logger,
			},
			Username:  config.String(envPrefix + ".user"),
			Password:  config.String(envPrefix + ".pass"),
			CACert:    caFileData,
			Transport: defaultTransportConfig,
		}
	case "production":
		generated = &elasticsearch.Config{
			Logger: &ESLogger{
				Logger: *logger,
			},
			CloudID:   config.String(envPrefix + ".cloudID"),
			APIKey:    config.String(envPrefix + ".apiKey"),
			Transport: defaultTransportConfig,
		}
	default:
		return nil, ErrInvalidConfig
	}

	return generated, nil
}
