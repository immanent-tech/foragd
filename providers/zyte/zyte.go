// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:generate go tool oapi-codegen -config zyte-cfg.yaml zyte.yaml
package zyte

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"
	"golang.org/x/time/rate"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Zyte.
	ConfigEnvPrefix = "ZYTE_"

	extractEndpoint = "https://api.zyte.com/v1/extract"
	rpmLimit        = 3000 // standard plan; use your own limit
	maxBanRetries   = 3    // 520/521/500: capped
	maxRateRetries  = 1000 // effectively "forever"; ctx deadline is the real bound
)

var ErrNotFound = errors.New("not found")

// Config is the configuration for Zyte.
type Config struct {
	// APIKey is the api key used to authorize requests with the zyte API.
	APIKey string `koanf:"apikey" validate:"required"`
}

var cfg Config

var loadConfig = sync.OnceValue(func() error {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return fmt.Errorf("load from envrionment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	slog.Info("Zyte config loaded.") //nolint:sloglint // we don't pass a context.
	return nil
})

var loadHTTPClient = sync.OnceValue(func() *resty.Client {
	// Optional client-side pacing, kept a bit under the key limit. 429s are still expected and normal; this just
	// reduces them.
	limiter := rate.NewLimiter(rate.Limit(float64(rpmLimit)*0.9/60), 50)

	var userAgent string
	if appCfg, err := config.LoadAppConfig(); err != nil {
		userAgent = "Foragd/Unknown (+https://foragd.app/policies/bot)"
	} else {
		userAgent = appCfg.GetAppName() + "/" + appCfg.GetAppVersion() + " (+https://foragd.app/policies/bot)"
	}
	client := client.New().
		SetHeader("User-Agent", userAgent).
		SetHeader("Content-Type", "application/json").
		SetRetryCount(maxRateRetries).
		SetTransport(&http.Transport{
			DialContext:         client.SecureDialer.DialContext,
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			MaxConnsPerHost:     200,
			IdleConnTimeout:     90 * time.Second,
		})

		// Runs before each attempt, so retries are paced too.
	client.OnBeforeRequest(func(_ *resty.Client, r *resty.Request) error {
		return limiter.Wait(r.Context())
	})

	client.AddRetryCondition(func(r *resty.Response, err error) bool {
		if r == nil { // network error
			return err != nil && attemptOf(r) <= maxBanRetries
		}
		switch r.StatusCode() {
		case 429, 503: // rate limiting: retry indefinitely
			return true
		case 520, 521, 500: // ban / temp error / service error: capped
			return r.Request.Attempt <= maxBanRetries
		}
		return false
	})

	// Custom backoff following the docs' example wait ranges.
	client.SetRetryAfter(func(_ *resty.Client, r *resty.Response) (time.Duration, error) {
		attempt := r.Request.Attempt
		if r.StatusCode() == 429 || r.StatusCode() == 503 {
			return jitter(rateLimitRange(attempt)), nil
		}
		return jitter(banRange(attempt)), nil
	})

	return client
})

func attemptOf(r *resty.Response) int {
	if r == nil || r.Request == nil {
		return 1
	}
	return r.Request.Attempt
}

// Rate-limit waits: 20-40s early, then 30s up to a growing max, capped at 630s.
func rateLimitRange(attempt int) (lo, hi time.Duration) {
	if attempt <= 2 {
		return 20 * time.Second, 40 * time.Second
	}
	h := math.Min(630, 30+math.Pow(2, float64(attempt)))
	return 30 * time.Second, time.Duration(h) * time.Second
}

// Unsuccessful-response waits: 3s up to a growing max, capped at 62s.
func banRange(attempt int) (lo, hi time.Duration) {
	h := math.Min(62, 7+math.Pow(2, float64(attempt)))
	return 3 * time.Second, time.Duration(h) * time.Second
}

func jitter(lo, hi time.Duration) time.Duration {
	return lo + time.Duration(rand.Int63n(int64(hi-lo)+1))
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}
