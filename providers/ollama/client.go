/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package ollama

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"
	"golang.org/x/oauth2"
)

const ConfigEnvPrefix = "OLLAMA_"

var defaultTimeout = 30 * time.Second

// Client contains the options for configuring the Ollama client.
type Client struct {
	// URL is the URL to the ollama server.
	URL string `koanf:"url" validate:"required,url"`
	// Model is the model to use.
	Model string `koanf:"model" validate:"required"`
	// BatchSize is the number of input texts to process at once.
	BatchSize int `koanf:"batchsize" validate:"omitempty,gt=0"`
	// KeepAlive is how long to keep a request alive.
	KeepAlive config.Duration `koanf:"keepalive"`

	httpClient *resty.Client `koanf:"-"`

	cancel context.CancelFunc `koanf:"-"`
	ctx    context.Context    `koanf:"-"`

	mu            sync.RWMutex       `koanf:"-"`
	tokenSource   oauth2.TokenSource `koanf:"-"`
	lastTokenTime time.Time          `koanf:"-"`
}

type Option func(*Client)

func WithURL(u string) Option                { return func(c *Client) { c.URL = u } }
func WithModel(m string) Option              { return func(c *Client) { c.Model = m } }
func WithBatchSize(n int) Option             { return func(c *Client) { c.BatchSize = n } }
func WithKeepAlive(d config.Duration) Option { return func(c *Client) { c.KeepAlive = d } }
func WithTimeout(d time.Duration) Option     { return func(c *Client) { defaultTimeout = d } }

func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		BatchSize: 50,
		KeepAlive: config.NewDuration(5 * time.Minute),
		httpClient: client.New().
			SetRetryCount(10).
			SetRetryMaxWaitTime(time.Minute),
	}

	// Load config from environment.
	if err := config.Load(ConfigEnvPrefix, &c); err != nil {
		return nil, fmt.Errorf("load config from envrionment: %w", err)
	}

	// Override config with specified options.
	for _, opt := range opts {
		opt(c)
	}

	if err := validation.Validate.Struct(c); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	// appCfg, err := config.LoadAppConfig()
	// if err != nil {
	// 	return nil, fmt.Errorf("load app config: %w", err)
	// }

	// Create background context for token refresh
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.cancel = cancel

	// if appCfg.IsProduction() {
	// 	var err error
	// 	c.tokenSource, err = idtoken.NewTokenSource(ctx, c.URL)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("generate token source: %w", err)
	// 	}
	// 	c.tokenSource = oauth2.ReuseTokenSource(nil, c.tokenSource)
	// }

	return c, nil
}

// Close will gracefully close the client.
func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	// Close the HTTP client if you're using it
	if c.httpClient != nil {
		// resty doesn't have Close(), but you can cancel all in-flight requests
	}
	return nil
}

// RefreshToken fetches a fresh token and applies it to the client.
func (c *Client) RefreshToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	token, err := c.tokenSource.Token()
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	c.httpClient.SetAuthToken(token.AccessToken)
	c.lastTokenTime = time.Now()

	return nil
}

// LastTokenAge returns how long ago the token was refreshed.
func (c *Client) LastTokenAge() time.Duration {
	return time.Since(c.lastTokenTime)
}
