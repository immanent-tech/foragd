// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package imgproxy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"

	"github.com/immanent-tech/go-base/config"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/validation"
)

const (
	configEnvPrefix = "IMGPROXY_"
)

type Config struct {
	Key    string `koanf:"key"    validate:"required,base64rawurl"`
	Salt   string `koanf:"salt"   validate:"required,base64rawurl"`
	Prefix string `koanf:"prefix" validate:"required,url"`
}

var loadConfig = sync.OnceValues(func() (*Config, error) {
	var cfg *Config
	// Load server config.
	if err := config.Load(configEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("load environment: %w", err)
	}

	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
})

func getKey() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.Key, nil
}

func getSalt() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	return cfg.Salt, nil
}

// GenerateImageProxyURL generates an image proxy URL for the given remote image URL. If a proxy URL cannot be
// generated, the original remote image URL is returned.
func GenerateImageProxyURL(ctx context.Context, url, props string) string {
	var keyBin, saltBin []byte
	var err error

	// Extract the key.
	key, err := getKey()
	if err != nil {
		slogctx.Error(ctx, "Get image proxy key failed",
			slog.Any("error", err))
		return url
	}
	if keyBin, err = hex.DecodeString(key); err != nil {
		return url
	}

	// Extract the salt.
	salt, err := getSalt()
	if err != nil {
		slogctx.Error(ctx, "Get image proxy salt failed",
			slog.Any("error", err))
		return url
	}
	if saltBin, err = hex.DecodeString(salt); err != nil {
		return url
	}

	encodedImageURL := base64.RawURLEncoding.EncodeToString([]byte(url))

	path := "/" + props + "/" + encodedImageURL

	mac := hmac.New(sha256.New, keyBin)
	mac.Write(saltBin)
	mac.Write([]byte(path))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return "/img-proxy/" + signature + path
}
