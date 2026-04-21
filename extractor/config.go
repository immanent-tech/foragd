// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package extractor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring the content extractor.
	ConfigEnvPrefix = "EXTRACTOR_"
)

// cfg is the content extractor config.
var cfg = Config{}

// allowedOutputFormats are the formats that the content extractor can generate.
var allowedOutputFormats = []string{"markdown", "txt", "html", "xml", "json"}

// ErrInvalidFormat indicates that format specified or requested is invalid.
var ErrInvalidFormat = errors.New("invalid format")

// Config contains the content extractor configuration options.
type Config struct {
	Key      string `koanf:"key"               validate:"required,base64rawurl"`
	Salt     string `koanf:"salt"              validate:"required,base64rawurl"`
	BaseURL  string `koanf:"baseurl"           validate:"required,url"`
	TokenTTL string `koanf:"token_ttl_seconds" validate:"omitempty"`
}

// LoadConfig loads the content extractor configuration and ensures this is only done one time, no matter how many times
// it is called.
var LoadConfig = sync.OnceValues(func() (*Config, error) {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
})

// GenerateExtractorURL takes the given URL and generates a new URL to proxy the request through the content extractor
// service.
func GenerateExtractorURL(originalURL, format string) (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return originalURL, fmt.Errorf("load config: %w", err)
	}

	ttl, err := time.ParseDuration(cfg.TokenTTL + "s")
	if err != nil {
		ttl = 60 * time.Second
	}
	expiry := strconv.FormatInt(time.Now().UTC().
		Add(ttl).
		Unix(), 10)

	if !slices.Contains(allowedOutputFormats, format) {
		return "", fmt.Errorf("%w: %s", ErrInvalidFormat, format)
	}

	payload := []byte(originalURL + "|" + format + "|" + expiry)

	mac := hmac.New(sha256.New, []byte(cfg.Key+cfg.Salt))
	mac.Write(payload)
	signature := mac.Sum(nil)

	token := base64.URLEncoding.EncodeToString(payload) + "." + base64.URLEncoding.EncodeToString(signature)

	proxyURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return originalURL, fmt.Errorf("parse proxy url: %w", err)
	}
	params := make(url.Values)
	params.Add("token", token)
	proxyURL.RawQuery = params.Encode()

	return proxyURL.String(), nil
}
