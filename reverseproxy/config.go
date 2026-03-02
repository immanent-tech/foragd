// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package reverseproxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/validation"
)

const (
	// ConfigEnvPrefix is the prefix applied to environment variables for configuring Auth0.
	ConfigEnvPrefix = config.ConfigEnvPrefix + "REVERSPROXY_"
)

// cfg is the server config with default values.
var cfg = Config{
	Port: 7000,
	Host: "localhost",
}

// Config contains the reverseproxy configuration options.
type Config struct {
	Port    uint64 `koanf:"port"    validate:"required,port"`
	Host    string `koanf:"host"    validate:"required,hostname|fqdn|ip"`
	Key     string `koanf:"key"     validate:"required,base64rawurl"`
	Salt    string `koanf:"salt"    validate:"required,base64rawurl"`
	BaseURL string `koanf:"baseurl" validate:"required,url"`
}

// LoadConfig loads the auth0 configuration and ensures this is only done
// one time, no matter how many times it is called.
var LoadConfig = sync.OnceValues(func() (*Config, error) {
	if err := config.Load(ConfigEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	// Load additional environment variables.
	if os.Getenv("PORT") != "" {
		if port, err := strconv.ParseUint(os.Getenv("PORT"), 10, 64); err != nil {
			return nil, fmt.Errorf("load port from envrionment: %w", err)
		} else {
			cfg.Port = port
		}
	}
	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("google: unable to validate config: %w", err)
	}
	return &cfg, nil
})

// GenerateProxyURL takes the given URL and generates a new URL to proxy the request through the reverse proxy service.
func GenerateProxyURL(originalURL string) (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return originalURL, fmt.Errorf("load config: %w", err)
	}
	expiry := strconv.FormatInt(time.Now().UTC().
		Add(3600*time.Second).
		Unix(), 10)

	encoded := []byte(originalURL + "|" + expiry + "|" + cfg.Salt)

	mac := hmac.New(sha256.New, []byte(cfg.Key))
	mac.Write(encoded)
	signature := hex.EncodeToString(mac.Sum(nil))

	proxyURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return originalURL, fmt.Errorf("parse proxy url: %w", err)
	}
	params := make(url.Values)
	params.Add("signature", signature)
	params.Add("url", originalURL)
	params.Add("expires", expiry)
	proxyURL.RawQuery = params.Encode()

	return proxyURL.String(), nil
}

// IsProxiedURL returns a boolean indicating whether the given URL is being proxied through the reverse proxy. If this
// cannot be determined, a non-nil error is also returned and the boolean status should be ignored.
func IsProxiedURL(value string) (bool, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}
	return strings.HasPrefix(value, cfg.BaseURL), nil
}
