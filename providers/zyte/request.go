// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package zyte

import (
	"context"
	"fmt"
	"slices"

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

// ExtractArticle attempts to extract an article from the given URL.
func ExtractArticle(ctx context.Context, url string, options ...RequestOption) (*Response, error) {
	req := NewRequest(url, options...)
	req.Article = new(true)
	req.ArticleOptions = &ArticleOptions{
		ExtractFrom: new(ExtractFromHttpResponseBody),
	}

	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	result := &Response{}
	errResult := &ResponseError{}

	switch resp, err := client.Load().R().
		SetContext(ctx).
		SetBasicAuth(cfg.APIKey, "").
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		SetError(errResult).
		// SetDebug(true).
		SetResult(result).
		Post("https://api.zyte.com/v1/extract"); {
	case err != nil:
		return nil, fmt.Errorf("extract article: %w", &ResponseError{Title: err.Error(), Status: resp.StatusCode()})
	case resp.IsError():
		return nil, fmt.Errorf("extract article %w", errResult)
	}

	return result, nil
}

func Proxy(ctx context.Context, url string, options ...RequestOption) (*Response, error) {
	req := NewRequest(url, options...)
	req.FollowRedirect = new(true)
	req.HttpResposeBody = new(true)

	if err := loadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	result := &Response{}
	errResult := &ResponseError{}

	switch resp, err := client.Load().R().
		SetContext(ctx).
		SetBasicAuth(cfg.APIKey, "").
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		SetError(errResult).
		// SetDebug(true).
		SetResult(result).
		Post("https://api.zyte.com/v1/extract"); {
	case err != nil:
		return nil, fmt.Errorf("proxy request: %w", &ResponseError{Title: err.Error(), Status: resp.StatusCode()})
	case resp.IsError():
		return nil, fmt.Errorf("proxy request %w", errResult)
	}

	return result, nil
}
