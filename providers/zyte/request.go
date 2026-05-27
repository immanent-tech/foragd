// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package zyte

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"slices"

	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/client"
)

type RequestOption func(*Request)

// NewRequest creates a new Zyte API request with the given options.
func NewRequest(url string, options ...RequestOption) *Request {
	req := &Request{
		URL: url,
	}
	for option := range slices.Values(options) {
		option(req)
	}

	return req
}

// WithResponseBody option specifies whether to include the raw response body in the response.
func WithResponseBody(value bool) RequestOption {
	return func(r *Request) {
		r.HttpResposeBody = &value
	}
}

// WithBrowserHTML option specifies whether to use a browser to get the page HTML.
func WithBrowserHTML(value bool) RequestOption {
	return func(r *Request) {
		r.BrowserHtml = &value
	}
}

// WithFollowRedirects option specifies whether any redirects should be followed.
func WithFollowRedirects(value bool) RequestOption {
	return func(r *Request) {
		r.FollowRedirect = &value
	}
}

// WithRequestMethod option specifies which request method to use. If not specified, this defaults to GET.
func WithRequestMethod(value RequestMethod) RequestOption {
	return func(r *Request) {
		r.HttpRequestMethod = &value
	}
}

// WithTag adds the given tag to the request.
func WithTag(key, value string) RequestOption {
	return func(r *Request) {
		if r.Tags == nil {
			r.Tags = make(map[string]string)
		}
		r.Tags[key] = value
	}
}

// ExtractArticle attempts to extract an article from the given URL.
func ExtractArticle(ctx context.Context, rawURL string, options ...RequestOption) (*Article, error) {
	sourceURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %s: %w", rawURL, err)
	}

	options = append(options,
		WithResponseBody(true),
		WithFollowRedirects(true),
	)
	req := NewRequest(sourceURL.String(), options...)

	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	result := &Response{}
	errResult := &ResponseError{}

	slogctx.FromCtx(ctx).Debug("Extracting article", slog.String("url", rawURL))

	switch resp, err := client.Load().R().
		SetContext(ctx).
		SetBasicAuth(cfg.APIKey, "").
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		SetError(errResult).
		// SetDebug(true).
		SetResult(result).
		Post(extractEndpoint); {
	case err != nil:
		return nil, fmt.Errorf("send api request: %w", &ResponseError{Title: err.Error(), Status: resp.StatusCode()})
	case resp.IsError():
		return nil, fmt.Errorf("send api request %w", errResult)
	}

	article, err := result.ExtractArticle()
	if err != nil {
		return nil, fmt.Errorf("extract article: %w", err)
	}

	return article, nil
}

// Proxy will reverse proxy the given URL through Zyte.
func Proxy(ctx context.Context, rawURL string, options ...RequestOption) (*Response, error) {
	sourceURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %s: %w", rawURL, err)
	}

	req := NewRequest(sourceURL.String(), options...)

	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	result := &Response{}
	errResult := &ResponseError{}

	slogctx.FromCtx(ctx).Debug("proxying request", slog.String("url", rawURL))

	switch resp, err := client.Load().R().
		SetContext(ctx).
		SetBasicAuth(cfg.APIKey, "").
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		SetError(errResult).
		// SetDebug(true).
		SetResult(result).
		Post(extractEndpoint); {
	case err != nil:
		return nil, fmt.Errorf("proxy request: %w", &ResponseError{Title: err.Error(), Status: resp.StatusCode()})
	case resp.IsError():
		return nil, fmt.Errorf("proxy request: %w", errResult)
	}

	return result, nil
}
