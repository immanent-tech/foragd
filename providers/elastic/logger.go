// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/logging"
)

var (
	// LogRequestBodyMaxSize is the maximum size of the request body to write to the log.
	LogRequestBodyMaxSize = 256 * 1024
	// LogResponseBodyMaxSize is the maximum size of the response body to write to the log.
	LogResponseBodyMaxSize = 256 * 1024
)

// Logger is a custom elastictransport.Logger.
type Logger struct {
	EnableRequestBody  bool
	EnableResponseBody bool
}

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.
func (l *Logger) LogRoundTrip(req *http.Request, res *http.Response, err error, start time.Time, dur time.Duration) error {
	// Extract some important values from the request and response.
	status := res.StatusCode
	path := req.URL.Path
	method := req.Method
	// Determine an appropriate log level based on the Elasticsearch response.
	var level slog.Level
	switch {
	case status >= http.StatusInternalServerError:
		level = slog.LevelError
		l.EnableRequestBody = true
		l.EnableResponseBody = true
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		level = slog.LevelWarn
		l.EnableRequestBody = true
		l.EnableResponseBody = true
	default:
		level = slog.LevelDebug
	}
	// Set base/common attributes.
	baseAttributes := []slog.Attr{}
	baseAttributes = append(baseAttributes,
		slog.Duration("took", dur),
	)
	if err != nil {
		baseAttributes = append(baseAttributes,
			slog.Any("error", err),
		)
	}
	if requestID := middleware.GetReqID(req.Context()); requestID != "" {
		baseAttributes = append(baseAttributes,
			slog.String("id", requestID),
		)
	}
	// Set request attributes.
	requestAttributes := []slog.Attr{
		slog.String("method", method),
		slog.String("path", path),
	}
	if route := chi.RouteContext(req.Context()).RoutePattern(); route != "" {
		requestAttributes = append(requestAttributes,
			slog.String("route", chi.RouteContext(req.Context()).RoutePattern()),
		)
	}
	if (logging.Level == logging.LevelTrace || l.RequestBodyEnabled()) && req != nil && req.Body != nil && req.Body != http.NoBody {
		var buf bytes.Buffer
		if req.GetBody != nil {
			b, _ := req.GetBody()
			buf.ReadFrom(b) //nolint:errcheck
		} else {
			buf.ReadFrom(req.Body) //nolint:errcheck
		}
		var body strings.Builder
		logBodyAsText(&body, &buf)
		requestAttributes = append(requestAttributes,
			slog.Int("length", int(res.ContentLength)),
			slog.String("body", body.String()))
	}
	// Set response attributes.
	responseAttributes := []slog.Attr{
		slog.Int("status", status),
	}
	// if (logging.Level == logging.LevelTrace || l.ResponseBodyEnabled()) && res != nil && res.Body != nil && res.Body != http.NoBody {
	// 	defer res.Body.Close()
	// 	var buf bytes.Buffer
	// 	buf.ReadFrom(res.Body)
	// 	var body strings.Builder
	// 	logBodyAsText(&body, &buf)
	// 	responseAttributes = append(responseAttributes,
	// 		slog.Int("length", int(res.ContentLength)),
	// 		slog.String("body", body.String()))
	// }
	// Define log attributes structure.
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
	// Write a log message.
	slogctx.FromCtx(req.Context()).LogAttrs(req.Context(), level, strconv.Itoa(status)+": "+http.StatusText(status), attributes...)
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

func logBodyAsText(dst io.Writer, body io.Reader) {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		s := scanner.Text()
		if s != "" {
			fmt.Fprintf(dst, "%s\n", s)
		}
	}
}

// func logBodyAsText(body *bytes.Buffer) string {
// 	// formatted, err := json.MarshalIndent(body.Bytes(), "", "  ")
// 	// if err != nil {
// 	// 	spew.Dump(err)
// 	// 	return ""
// 	// }
// 	return string(pretty.Color(body.Bytes(), nil))
// 	// scanner := bufio.NewScanner(body)
// 	// for scanner.Scan() {
// 	// 	s := scanner.Text()
// 	// 	if s != "" {
// 	// 		return s
// 	// 	}
// 	// }
// 	// return ""
// }
