// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package client

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/immanent-tech/foragd/config"
)

var (
	// Set the User-Agent string to be used for underlying requests to fetch feeds and content.
	UserAgent = config.AppName + "/" + config.Version + " (+https://foragd.app/policies/bot)"
	// DefaultHTTPRequestTimeout is the maximum time allowed for a background HTTP request to execute.
	DefaultHTTPRequestTimeout = 45 * time.Second
	// DefaultRequestRetries is the default number of retries for API requests.
	DefaultRequestRetries = 3
)

var client *resty.Client

var LoadHTTPClient = sync.OnceValue(func() *resty.Client {
	client = resty.New().
		SetHeader("User-Agent", UserAgent).
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "gzip, deflate")
	return client
})

// limitedReader wraps a reader and stops after the </head> tag or a byte limit.
// This avoids downloading the entire page body.
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

func (h *HeadReader) Read(p []byte) (int, error) {
	if h.done {
		return 0, io.EOF
	}
	if h.total >= h.maxRead {
		return 0, io.EOF
	}
	n, err := h.r.Read(p)
	h.total += n
	// Look for </head> in what we just read to stop early
	chunk := strings.ToLower(string(p[:n]))
	if idx := strings.Index(chunk, "</head>"); idx != -1 {
		h.done = true
		return idx + len("</head>"), io.EOF
	}
	return n, err
}
