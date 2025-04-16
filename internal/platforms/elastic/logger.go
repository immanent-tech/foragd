// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/go-chi/chi/v5/middleware"
)

var _ elastictransport.Logger = (*Logger)(nil)

var (
	// LogRequestBodyMaxSize is the maximum size of the request body to write to the log.
	LogRequestBodyMaxSize = 256 * 1024
	// LogResponseBodyMaxSize is the maximum size of the response body to write to the log.
	LogResponseBodyMaxSize = 256 * 1024
)

// Logger is a custom elastictransport.Logger.
type Logger struct {
	logger             *slog.Logger
	EnableRequestBody  bool
	EnableResponseBody bool
}

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.
//
//nolint:funlen
func (l *Logger) LogRoundTrip(req *http.Request, res *http.Response, err error, start time.Time, dur time.Duration) error {
	baseAttributes := []slog.Attr{}

	path := req.URL.Path
	method := req.Method
	params := req.URL.Query()
	end := time.Now()
	latency := end.Sub(start)

	requestAttributes := []slog.Attr{
		slog.String("method", method),
		slog.String("path", path),
		slog.Any("params", params),
	}

	status := res.StatusCode
	responseAttributes := []slog.Attr{
		// slog.Time("time", end.UTC()),
		slog.Duration("latency", latency),
		slog.Int("status", status),
	}

	if requestID := middleware.GetReqID(req.Context()); requestID != "" {
		requestAttributes = append(requestAttributes,
			slog.String("id", requestID),
		)
		responseAttributes = append(responseAttributes,
			slog.String("id", requestID),
		)
	}

	baseAttributes = append(baseAttributes,
		slog.Duration("took", dur),
	)
	if err != nil {
		baseAttributes = append(baseAttributes,
			slog.Any("error", err),
		)
	}

	// request body
	if l.RequestBodyEnabled() && req != nil && req.Body != nil && req.Body != http.NoBody {
		var buf bytes.Buffer
		if req.GetBody != nil {
			b, _ := req.GetBody()
			buf.ReadFrom(b) //nolint:errcheck
		} else {
			buf.ReadFrom(req.Body) //nolint:errcheck
		}
		requestAttributes = append(requestAttributes,
			slog.Int("length", int(res.ContentLength)),
			slog.String("body", logBodyAsText(&buf)))
	}

	// response body
	if l.ResponseBodyEnabled() && res != nil && res.Body != nil && res.Body != http.NoBody {
		defer res.Body.Close() //nolint:errcheck
		var buf bytes.Buffer
		buf.ReadFrom(res.Body) //nolint:errcheck
		requestAttributes = append(requestAttributes,
			slog.Int("length", int(res.ContentLength)),
			slog.String("body", logBodyAsText(&buf)))
	}

	attributes := append(
		[]slog.Attr{
			{
				Key:   "request",
				Value: slog.GroupValue(requestAttributes...),
			},
			{
				Key:   "response",
				Value: slog.GroupValue(responseAttributes...),
			},
		},
		baseAttributes...,
	)

	level := slog.LevelDebug
	if status >= http.StatusInternalServerError {
		level = slog.LevelError
	} else if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
		level = slog.LevelWarn
	}

	l.logger.LogAttrs(req.Context(), level, strconv.Itoa(status)+": "+http.StatusText(status), attributes...)
	return err
}

// RequestBodyEnabled makes the client pass a copy of request body to the logger.
func (l *Logger) RequestBodyEnabled() bool {
	return l.EnableRequestBody
}

// ResponseBodyEnabled makes the client pass a copy of response body to the logger.
func (l *Logger) ResponseBodyEnabled() bool {
	return l.EnableResponseBody
}

func logBodyAsText(body io.Reader) string {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		s := scanner.Text()
		if s != "" {
			return s
		}
	}
	return ""
}
