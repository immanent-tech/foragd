// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package zyte

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"

	"github.com/immanent-tech/go-syndication/opengraph"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/client"
	"github.com/immanent-tech/foragd/pkg/formats/html"
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
func ExtractArticle(ctx context.Context, rawURL string, options ...RequestOption) (*Response, error) {
	sourceURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %s: %w", err)
	}

	req := NewRequest(sourceURL.String(), options...)
	req.Article = new(true)
	req.ArticleOptions = &ExtractOptions{
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
		Post(extractEndpoint); {
	case err != nil:
		return nil, fmt.Errorf("extract article: %w", &ResponseError{Title: err.Error(), Status: resp.StatusCode()})
	case resp.IsError():
		return nil, fmt.Errorf("extract article %w", errResult)
	}

	return result, nil
}

// ExtractMainImage will attempt to extract a URL to what is likely the "main" image of a page (i.e., typically used on
// article/post pages).
func ExtractMainImage(ctx context.Context, rawURL string) (string, error) {
	resp, err := Proxy(ctx, rawURL)
	if err != nil || resp.HttpResponseBody == nil {
		return "", fmt.Errorf("retrieve content: %w", err)
	}

	// Create a buffer for the feed data.
	pageBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("get buffer failed")
	}
	pageBuf.Reset()
	defer bufPool.Put(pageBuf)
	if _, err := pageBuf.WriteString(*resp.HttpResponseBody); err != nil {
		return "", fmt.Errorf("write buffer: %w", err)
	}

	var foundURL string

	// Try to parse opengraph data out of the page content.
	if og, err := opengraph.ParseBytes(pageBuf.Bytes()); err != nil {
		slogctx.FromCtx(ctx).Debug("Could not parse opengraph data for URL.",
			slog.String("url", rawURL),
			slog.Any("error", err))
	} else {
		foundURL = og.Image
	}

	// Try to find the "main" image in the page content.
	if foundURL == "" {
		foundURL, _ = html.FindMainImage(pageBuf.Bytes(), rawURL)
	}

	// Parse the found URL.
	imgURL, err := url.Parse(foundURL)
	if err != nil {
		return foundURL, fmt.Errorf("parse image URL %q: %w", foundURL, err)
	}

	// If it is not an absolute URL, resolve it relative to the page URL.
	if !imgURL.IsAbs() {
		sourceURL, _ := url.Parse(rawURL)
		return sourceURL.ResolveReference(imgURL).String(), nil
	}

	return imgURL.String(), nil
}

// ExtractFavicon will attempt to extract a URL to what is likely the favicon of a page.
func ExtractFavicon(ctx context.Context, rawURL string) (string, error) {
	resp, err := Proxy(ctx, rawURL)
	if err != nil || resp.HttpResponseBody == nil {
		return "", fmt.Errorf("retrieve content: %w", err)
	}

	// Create a buffer for the feed data.
	pageBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("get buffer failed")
	}
	pageBuf.Reset()
	defer bufPool.Put(pageBuf)
	if _, err := pageBuf.WriteString(*resp.HttpResponseBody); err != nil {
		return "", fmt.Errorf("write buffer: %w", err)
	}

	_, faviconURL, _, err := html.FindFavicon(pageBuf.Bytes(), rawURL)
	if err != nil {
		return "", fmt.Errorf("find favicon: %w", err)
	}

	// Parse the found URL.
	imgURL, err := url.Parse(faviconURL)
	if err != nil {
		return faviconURL, fmt.Errorf("parse favicon URL %q: %w", faviconURL, err)
	}

	// If it is not an absolute URL, resolve it relative to the page URL.
	if !imgURL.IsAbs() {
		sourceURL, _ := url.Parse(rawURL)
		return sourceURL.ResolveReference(imgURL).String(), nil
	}

	return imgURL.String(), nil
}

// Proxy will reverse proxy the given URL through Zyte.
func Proxy(ctx context.Context, rawURL string, options ...RequestOption) (*Response, error) {
	sourceURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %s: %w", err)
	}

	req := NewRequest(sourceURL.String(), options...)
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
		Post(extractEndpoint); {
	case err != nil:
		return nil, fmt.Errorf("proxy request: %w", &ResponseError{Title: err.Error(), Status: resp.StatusCode()})
	case resp.IsError():
		return nil, fmt.Errorf("proxy request %w", errResult)
	}

	return result, nil
}
