// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/opengraph"

	"github.com/immanent-tech/foragd/config"
	"github.com/immanent-tech/foragd/pkg/formats/html"
	"github.com/immanent-tech/foragd/validation"
)

var (
	// UserAgent is the string which the `User-Agent` request header will be set to for underlying requests to fetch
	// feeds and content.
	UserAgent = config.AppName + "/" + config.Version + " (+https://foragd.app/policies/bot)"
	// DefaultHTTPRequestTimeout is the maximum time allowed for a background HTTP request to execute.
	DefaultHTTPRequestTimeout = 45 * time.Second
	// DefaultRequestRetries is the default number of retries for API requests.
	DefaultRequestRetries = 3
)

var client *resty.Client

var Load = sync.OnceValue(func() *resty.Client {
	client = resty.New().
		SetHeader("User-Agent", UserAgent).
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "gzip, deflate")
	return client
})

// HeadReader wraps a reader and stops after the </head> tag or a byte limit. This avoids downloading the entire page
// body.
type HeadReader struct {
	r       io.Reader
	buf     []byte
	done    bool
	total   int
	maxRead int
}

func NewHeadReader(r io.Reader, maxBytes int) *HeadReader {
	return &HeadReader{r: r, maxRead: maxBytes}
}

func (h *HeadReader) Read(page []byte) (int, error) {
	if h.done {
		return 0, io.EOF
	}
	if h.total >= h.maxRead {
		return 0, io.EOF
	}
	n, err := h.r.Read(page)
	h.total += n
	// Look for </head> in what we just read to stop early
	chunk := strings.ToLower(string(page[:n]))
	if idx := strings.Index(chunk, "</head>"); idx != -1 {
		h.done = true
		return idx + len("</head>"), io.EOF
	}
	return n, fmt.Errorf("read header: %w", err)
}

// ExtractMainContent extracts the main content from the page at the given URL, using the readability package.
func ExtractMainContent(ctx context.Context, page string) (string, error) {
	pageURL, err := url.Parse(page)
	if err != nil {
		return "", fmt.Errorf("parse page url: %w", err)
	}

	// Get the page data.
	resp, err := Load().R().
		SetContext(ctx).
		Get(pageURL.String())
	if err != nil {
		return "", fmt.Errorf("get page data: %w", err)
	}
	if resp.IsError() {
		return "", fmt.Errorf("get page data: %v", resp.Error())
	}

	// Write the page data to a buffer.
	respBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("unable to allocate resp buffer")
	}
	defer func() {
		respBuf.Reset()
		bufPool.Put(respBuf)
	}()
	_, err = respBuf.Write(resp.Body())
	if err != nil {
		return "", fmt.Errorf("write page data to buffer: %w", err)
	}

	// Attempt to extract main content from the page data.
	remote, err := readability.FromReader(respBuf, pageURL)
	if err != nil {
		return "", fmt.Errorf("extract article from url %s: %w", pageURL, err)
	}
	articleBuf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("unable to allocate article buffer")
	}
	defer func() {
		articleBuf.Reset()
		bufPool.Put(articleBuf)
	}()
	if err := remote.RenderHTML(articleBuf); err != nil {
		return "", fmt.Errorf("render article html: %w", err)
	}

	// Sanitise the result.
	content := validation.SanitizeString(articleBuf.String())
	return content, nil
}

// ExtractMainImage will try to find an image to represent the feed. Useful to call if the feed does not define an image
// itself.
func ExtractMainImage(ctx context.Context, page string) (string, error) {
	// Retrieve the content from the feed's site page.
	resp, err := Load().R().SetContext(ctx).Get(page)
	if err != nil {
		return "", fmt.Errorf("get url: %w", err)
	}
	if resp.IsError() {
		return "", fmt.Errorf("%s: %s", resp.Status(), resp.Error())
	}

	// Try to parse opengraph data out of the page content.
	if og, err := opengraph.ParseBytes(resp.Body()); err != nil {
		slogctx.FromCtx(ctx).Debug("Could not parse opengraph data for URL.",
			slog.String("url", page),
			slog.Any("error", err))
	} else {
		return og.Image, nil
	}

	// Try to find the "main" image in the page content.
	if imgURL, _ := html.FindMainImage(resp.Body(), page); imgURL != "" {
		return imgURL, nil
	}

	// Try to find a favicon to use.
	if _, url, _, err := html.FindFavicon(resp.Body(), page); err != nil {
		slogctx.FromCtx(ctx).Debug("Could not find favicon for URL.",
			slog.String("url", page),
			slog.Any("error", err))
	} else {
		return url, nil
	}

	return "", fmt.Errorf("%q: no image found", page)
}

var bufPool = sync.Pool{
	New: func() any {
		var buf bytes.Buffer
		return &buf
	},
}
