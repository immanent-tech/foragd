// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/goforj/godump"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/google/go-github/v75/github"
	"github.com/jferrl/go-githubauth"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/oauth2"

	"github.com/immanent-tech/foragd/models"
)

var client *github.Client

// Connect will create a client connection with the GitHub API. The connection will be cached for re-use. It is safe to
// call multiple times, with subsequent calls being no-ops.
var Connect = sync.OnceValue(func() error {
	err := LoadConfigOnce()
	if err != nil {
		return fmt.Errorf("unable to create github client: %w", err)
	}

	// Generate app installation token.
	//
	// https://github.com/jferrl/go-githubauth?tab=readme-ov-file#generate-github-app-installation-token
	data, err := base64.StdEncoding.DecodeString(cfg.Key)
	if err != nil {
		return fmt.Errorf("unable to use github api: %w", err)
	}
	appTokenSource, err := githubauth.NewApplicationTokenSource(cfg.ClientID, data)
	if err != nil {
		return fmt.Errorf("unable to use github api: %w", err)
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
	slog.Debug("GitHub client connected.")
	return nil
})

// CreateObjectIssue creates a new issue in Github about problems with a particular object reported by a user.
func CreateObjectIssue(ctx context.Context, details *models.ObjectIssueRequest) error {
	title := "Object Issue: " + string(details.Object)
	labels := []string{"subscription"}
	// Build issue body.
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("Object ID: " + details.ObjectID)
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Page URL: " + details.PageUrl)
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Issues:")
	bodyBuilder.WriteRune('\n')
	if details.MangledContent {
		bodyBuilder.WriteString("- Mangled content.")
		bodyBuilder.WriteRune('\n')
	}
	if details.MissingImage {
		bodyBuilder.WriteString("- Missing image.")
		bodyBuilder.WriteRune('\n')
	}
	if details.Duplicate {
		bodyBuilder.WriteString("- Duplicate object.")
		bodyBuilder.WriteRune('\n')
	}
	if details.Details != "" {
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString("Details:")
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString(details.Details)
		bodyBuilder.WriteRune('\n')
	}
	body := bodyBuilder.String()
	issueDetails := github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	}
	ctx = context.WithValue(ctx, github.BypassRateLimitCheck, true)
	issue, response, err := client.Issues.Create(ctx, "immanent-tech", "foragd", &issueDetails)
	var priRateErr *github.RateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit primary rate limit.",
			slog.Int("remaining", priRateErr.Rate.Remaining),
			slog.Int("limit", priRateErr.Rate.Limit))
	}
	var secRateErr *github.AbuseRateLimitError
	if errors.As(err, &secRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit secondary rate limit.",
			slog.Duration("retry_after", *secRateErr.RetryAfter))
	}
	godump.Dump(issue, response)
	if err != nil {
		return fmt.Errorf("unable to create subscription issue: %w", err)
	}
	return nil
}

// CreateIssue creates a new issue in Github about problems with the app reported by a user.
func CreateIssue(ctx context.Context, details *models.IssueRequest) error {
	user, err := models.UserFromCtx(ctx)
	if err != nil {
		return fmt.Errorf("unable to create app issue: %w", err)
	}
	title := "App Issue reported by " + user.GetNickname()
	labels := []string{"subscription"}
	// Build issue body.
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("User ID: " + user.GetID())
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Page URL: " + details.PageUrl)
	bodyBuilder.WriteRune('\n')
	if details.Details != "" {
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString("Details:")
		bodyBuilder.WriteRune('\n')
		bodyBuilder.WriteString(details.Details)
		bodyBuilder.WriteRune('\n')
	}

	body := bodyBuilder.String()
	issueDetails := github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	}
	ctx = context.WithValue(ctx, github.BypassRateLimitCheck, true)
	issue, response, err := client.Issues.Create(ctx, "immanent-tech", "foragd", &issueDetails)
	var priRateErr *github.RateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit primary rate limit.",
			slog.Int("remaining", priRateErr.Rate.Remaining),
			slog.Int("limit", priRateErr.Rate.Limit))
	}
	var secRateErr *github.AbuseRateLimitError
	if errors.As(err, &secRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit secondary rate limit.",
			slog.Duration("retry_after", *secRateErr.RetryAfter))
	}
	godump.Dump(issue, response)
	if err != nil {
		return fmt.Errorf("unable to create subscription issue: %w", err)
	}
	return nil
}
