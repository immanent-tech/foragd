// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/google/go-github/v75/github"
	"github.com/jferrl/go-githubauth"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/oauth2"
)

var client *github.Client

// Connect will create a client connection with the GitHub API. The connection will be cached for re-use. It is safe to
// call multiple times, with subsequent calls being no-ops.
func Connect(ctx context.Context) error {
	return sync.OnceValue(func() error {
		// Load config.
		if err := loadConfigOnce(); err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		// Generate app installation token.
		//
		// https://github.com/jferrl/go-githubauth?tab=readme-ov-file#generate-github-app-installation-token
		data, err := base64.StdEncoding.DecodeString(cfg.Key)
		if err != nil {
			return fmt.Errorf("decode app installation token: %w", err)
		}
		appTokenSource, err := githubauth.NewApplicationTokenSource(cfg.ClientID, data)
		if err != nil {
			return fmt.Errorf("generate app authentication token: %w", err)
		}
		installationTokenSource := githubauth.NewInstallationTokenSource(int64(cfg.InstallationID), appTokenSource)

		// Set up client with token and rate-limit handling.
		httpClient := oauth2.NewClient(context.Background(), installationTokenSource)
		rateLimiter := github_ratelimit.NewClient(nil)
		rateLimiter.Transport = httpClient.Transport
		rateLimiter.Jar = httpClient.Jar
		rateLimiter.CheckRedirect = httpClient.CheckRedirect
		rateLimiter.Timeout = httpClient.Timeout
		client = github.NewClient(rateLimiter)
		slogctx.FromCtx(ctx).Debug("GitHub client connected.")
		return nil
	})()
}
