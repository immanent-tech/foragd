// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package client

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/immanent-tech/foragd/config"
)

var (
	// UserAgent is the string which the `User-Agent` request header will be set to for underlying requests to fetch
	// feeds and content.
	UserAgent = config.AppName + "/" + config.GetVersion() + " (+https://foragd.app/policies/bot)"
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
