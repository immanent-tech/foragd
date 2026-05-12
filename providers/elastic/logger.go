// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package elastic

import (
	"bytes"
	"encoding/json"
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

// Logger is a custom elastictransport.Logger.
type Logger struct {
	EnableRequestBody  bool
	EnableResponseBody bool
}

var quoteReplacer = strings.NewReplacer(`"`, `'`)

// LogRoundTrip should not modify the request or response, except for consuming and closing the body.
// Implementations have to check for nil values in request and response.
func (l *Logger) LogRoundTrip(
	req *http.Request,
	res *http.Response,
	err error,
	_ time.Time,
	dur time.Duration,
) error {
	// Extract some important values from the request and response.
	status := res.StatusCode
	path := req.URL.Path
	method := req.Method
	// Determine an appropriate log level based on the Elasticsearch response.
	var level slog.Level
	switch {
	case status >= http.StatusInternalServerError:
		// All 5XX responses are ERROR level.
		level = slog.LevelError
		l.EnableRequestBody = true
		l.EnableResponseBody = true
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError && status != http.StatusNotFound:
		// All 4XX responsese excluding 404 are WARN level.
		level = slog.LevelWarn
		l.EnableRequestBody = true
		l.EnableResponseBody = true
	default:
		// Default TRACE level.
		level = logging.LevelTrace
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
	if (logging.Level == logging.LevelTrace || l.RequestBodyEnabled()) && req != nil && req.Body != nil &&
		req.Body != http.NoBody {
		var (
			buf bytes.Buffer
		)
		if req.GetBody != nil {
			b, _ := req.GetBody()
			_, err = buf.ReadFrom(b)
		} else {
			_, err = buf.ReadFrom(req.Body)
		}

		requestAttributes = append(requestAttributes, slog.String("body", buf.String()))
	}
	// if req != nil && req.Body != nil && req.Body != http.NoBody {
	// 	var (
	// 		buf bytes.Buffer
	// 	)
	// 	if req.GetBody != nil {
	// 		b, _ := req.GetBody()
	// 		_, err = buf.ReadFrom(b)
	// 	} else {
	// 		_, err = buf.ReadFrom(req.Body)
	// 	}
	// 	godump.Dump(req.URL.Path, quoteReplacer.Replace(buf.String()))
	// }

	// Set response attributes.
	responseAttributes := []slog.Attr{
		slog.Int("status", status),
	}
	// if (logging.Level == logging.LevelTrace || l.ResponseBodyEnabled()) && res != nil && res.Body != nil &&
	// 	res.Body != http.NoBody {
	// 	defer res.Body.Close()
	// 	var buf bytes.Buffer
	// 	buf.ReadFrom(res.Body)
	// 	res.Body = io.NopCloser(bytes.NewBuffer(buf.Bytes()))
	// 	var body strings.Builder
	// 	responseAttributes = append(responseAttributes,
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
	slogctx.FromCtx(req.Context()).
		LogAttrs(req.Context(), level, strconv.Itoa(status)+": "+http.StatusText(status), attributes...)
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

func jsonToSlogAttr(key string, data []byte) slog.Attr {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return slog.String(key, string(data)) // fallback
	}
	return slog.Group(key, mapToArgs(m)...)
}

func mapToArgs(m map[string]any) []any {
	args := make([]any, 0, len(m)*2)
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			args = append(args, slog.Group(k, mapToArgs(val)...))
		default:
			args = append(args, slog.Any(k, v))
		}
	}
	return args
}
