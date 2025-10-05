// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/goforj/godump"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/google/go-github/v75/github"
	"github.com/jferrl/go-githubauth"
	slogctx "github.com/veqryn/slog-context"
	"golang.org/x/oauth2"

	"github.com/immanent-tech/foragd/models"
)

// Client provides access to the Github API.
type Client struct {
	*github.Client
}

// NewClient creates a new client for using the Github API.
func NewClient(ctx context.Context) (*Client, error) {
	err := LoadConfigOnce()
	if err != nil {
		return nil, fmt.Errorf("unable to create github client: %w", err)
	}

	// Generate app installation token.
	//
	// https://github.com/jferrl/go-githubauth?tab=readme-ov-file#generate-github-app-installation-token
	pemKey, err := os.ReadFile(cfg.Key)
	if err != nil {
		return nil, fmt.Errorf("unable to use github api: %w", err)
	}
	appTokenSource, err := githubauth.NewApplicationTokenSource[string](cfg.ClientID, pemKey)
	if err != nil {
		return nil, fmt.Errorf("unable to use github api: %w", err)
	}
	installationTokenSource := githubauth.NewInstallationTokenSource(int64(cfg.InstallationID), appTokenSource)

	// Set up client with token and rate-limit handling.
	httpClient := oauth2.NewClient(context.Background(), installationTokenSource)
	rateLimiter := github_ratelimit.NewClient(nil)
	rateLimiter.Transport = httpClient.Transport
	rateLimiter.Jar = httpClient.Jar
	rateLimiter.CheckRedirect = httpClient.CheckRedirect
	rateLimiter.Timeout = httpClient.Timeout
	githubClient := github.NewClient(rateLimiter)
	return &Client{Client: githubClient}, nil
}

// CreateSubscriptionIssue creates a new issue in Github about problems with a subscription reported by a user.
func (c *Client) CreateSubscriptionIssue(ctx context.Context, subscription *models.Subscription, details *models.SubscriptionIssue) error {
	title := "Feed Issue: " + subscription.GetTitle()
	labels := []string{"subscription"}
	// Build issue body.
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("Subscription ID: " + subscription.GetID())
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Feed ID: " + subscription.GetFeedID())
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Feed URLs:")
	bodyBuilder.WriteRune('\n')
	for url := range slices.Values(subscription.Feed.SourceURLs) {
		bodyBuilder.WriteString(url)
		bodyBuilder.WriteRune('\n')
	}
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Issues:")
	bodyBuilder.WriteRune('\n')
	if details.MangledContent {
		bodyBuilder.WriteString("- Mangled text.")
		bodyBuilder.WriteRune('\n')
	}
	if details.MissingImage {
		bodyBuilder.WriteString("- Missing image.")
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
	issue, response, err := c.Issues.Create(ctx, "immanent-tech", "foragd", &issueDetails)
	var priRateErr *github.RateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit primary rate limit.",
			slog.Int("remaining", priRateErr.Rate.Remaining),
			slog.Int("limit", priRateErr.Rate.Limit))
	}
	var secRateErr *github.AbuseRateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit secondary rate limit.",
			slog.Duration("retry_after", *secRateErr.RetryAfter))
	}
	godump.Dump(issue, response)
	if err != nil {
		return fmt.Errorf("unable to create subscription issue: %w", err)
	}
	return nil
}

// CreateArticleIssue creates a new issue in Github about problems with an article reported by a user.
func (c *Client) CreateArticleIssue(ctx context.Context, article *models.Article, details *models.ArticleIssue) error {
	title := "Feed Issue: " + article.GetTitle()
	labels := []string{"subscription"}
	// Build issue body.
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("Page URL: " + details.PageUrl)
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Subscription ID: " + article.GetSubscriptionID())
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Item ID: " + article.GetID())
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Source type: " + string(article.Item.SourceType))
	bodyBuilder.WriteRune('\n')
	bodyBuilder.WriteString("Issues:")
	bodyBuilder.WriteRune('\n')
	if details.MangledContent {
		bodyBuilder.WriteString("- Mangled text.")
		bodyBuilder.WriteRune('\n')
	}
	if details.MissingImage {
		bodyBuilder.WriteString("- Missing image.")
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
	issue, response, err := c.Issues.Create(ctx, "immanent-tech", "foragd", &issueDetails)
	var priRateErr *github.RateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit primary rate limit.",
			slog.Int("remaining", priRateErr.Rate.Remaining),
			slog.Int("limit", priRateErr.Rate.Limit))
	}
	var secRateErr *github.AbuseRateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit secondary rate limit.",
			slog.Duration("retry_after", *secRateErr.RetryAfter))
	}
	godump.Dump(issue, response)
	if err != nil {
		return fmt.Errorf("unable to create subscription issue: %w", err)
	}
	return nil
}

// CreatePageIssue creates a new issue in Github about problems with the app reported by a user.
func (c *Client) CreatePageIssue(ctx context.Context, details *models.PageIssue) error {
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
	issue, response, err := c.Issues.Create(ctx, "immanent-tech", "foragd", &issueDetails)
	var priRateErr *github.RateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit primary rate limit.",
			slog.Int("remaining", priRateErr.Rate.Remaining),
			slog.Int("limit", priRateErr.Rate.Limit))
	}
	var secRateErr *github.AbuseRateLimitError
	if errors.As(err, &priRateErr) {
		slogctx.FromCtx(ctx).Warn("Hit secondary rate limit.",
			slog.Duration("retry_after", *secRateErr.RetryAfter))
	}
	godump.Dump(issue, response)
	if err != nil {
		return fmt.Errorf("unable to create subscription issue: %w", err)
	}
	return nil
}
