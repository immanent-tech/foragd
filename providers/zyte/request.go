// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package zyte

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"
	slogctx "github.com/veqryn/slog-context"
)

var (
	// DefaultRequestTimeout is the timeout for extraction requests. This is greater than proxy requests as these
	// often take more time.
	DefaultRequestTimeout = time.Minute
)

type RequestOption func(*Request)

// NewRequest creates a new Zyte API request with the given options.
func NewRequest(url string, options ...RequestOption) *Request {
	req := &Request{
		URL:     url,
		Timeout: &DefaultRequestTimeout,
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

// WithExtractFrom option sets the extraction source.
func WithExtractFrom(from ExtractFrom) RequestOption {
	return func(r *Request) {
		switch from {
		case ExtractFromBrowserHtml:
			r.BrowserHtml = new(true)
		case ExtractFromHttpResponseBody:
			fallthrough
		default:
			r.HttpResposeBody = new(true)
		}
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

// WithTimeout option sets the timeout after which a request will be cancelled. If not set, an appropriate default value
// will be used.
func WithTimeout(timeout time.Duration) RequestOption {
	return func(r *Request) {
		r.Timeout = &timeout
	}
}

// AsArticle will configure Zyte to perform automatic article extraction.
func AsArticle(opts *ExtractOptions) RequestOption {
	return func(r *Request) {
		r.Article = new(true)
		if opts != nil {
			r.ArticleOptions = opts
		}
	}
}

// AsArticleList will configure Zyte to perform automatic article list extraction.
func AsArticleList(opts *ExtractOptions) RequestOption {
	return func(r *Request) {
		r.ArticleList = new(true)
		if opts != nil {
			r.ArticleListOptions = opts
		}
	}
}

// ExtractArticle attempts to extract an article from the given URL.
func ExtractArticle(ctx context.Context, rawURL string, options ...RequestOption) (*Article, error) {
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

	ctx, cancelFunc := context.WithTimeout(ctx, *req.Timeout)
	defer cancelFunc()

	slogctx.FromCtx(ctx).Debug("Extracting article", slog.String("url", rawURL))

	client, err := client.Load()
	if err != nil {
		return nil, fmt.Errorf("load http client: %w", err)
	}
	switch resp, err := client.
		// Add retry logic for 429 and 520 responses as per Zyte API guidelines.
		SetRetryCount(3).
		AddRetryCondition(
			func(r *resty.Response, _ error) bool {
				return r.StatusCode() == http.StatusTooManyRequests || r.StatusCode() == 520
			},
		).
		R().
		SetContext(ctx).
		SetHeader("User-Agent", config.GetAppName()+"/"+config.GetVersion()+" (+https://foragd.app/policies/bot)").
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
		return nil, fmt.Errorf(
			"extract article: %w",
			&ResponseError{Title: err.Error(), Status: http.StatusUnprocessableEntity},
		)
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

	ctx, cancelFunc := context.WithTimeout(ctx, *req.Timeout)
	defer cancelFunc()

	slogctx.FromCtx(ctx).Debug("proxying request", slog.String("url", rawURL))

	client, err := client.Load()
	if err != nil {
		return nil, fmt.Errorf("load http client: %w", err)
	}
	switch resp, err := client.
		// Add retry logic for 429 and 520 responses as per Zyte API guidelines.
		SetRetryCount(3).
		AddRetryCondition(
			func(r *resty.Response, _ error) bool {
				return r.StatusCode() == http.StatusTooManyRequests || r.StatusCode() == 520
			},
		).
		R().
		SetContext(ctx).
		SetHeader("User-Agent", config.GetAppName()+"/"+config.GetVersion()+" (+https://foragd.app/policies/bot)").
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
