// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//revive:disable:unused-receiver
package elastic

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	LevelTrace = slog.Level(-8)
	LevelFatal = slog.Level(12)
)

type ESLogger struct {
	slog.Logger
}

func (l *ESLogger) LogRoundTrip(req *http.Request, res *http.Response, _ error, _ time.Time, dur time.Duration) error {
	var (
		nReq int64
		nRes int64
	)

	var reqBody bytes.Buffer

	// Count number of bytes in request and response.
	//
	if req != nil && req.Body != nil && req.Body != http.NoBody {
		nReq, _ = io.Copy(&reqBody, req.Body) //nolint:errcheck
	}

	if res != nil && res.Body != nil && res.Body != http.NoBody {
		nRes, _ = io.Copy(io.Discard, res.Body) //nolint:errcheck
	}

	// Log event.
	//
	l.Debug("Round Trip Stats.",
		slog.String("server", req.URL.Host),
		slog.String("path", req.URL.Path),
		slog.String("method", req.Method),
		slog.Int("status_code", res.StatusCode),
		slog.Duration("duration", dur),
		slog.Int64("req_bytes", nReq),
		slog.Int64("res_bytes", nRes),
		// slog.String("body", reqBody.String()),
	)

	return nil
}

// RequestBodyEnabled makes the client pass request body to logger.
func (l *ESLogger) RequestBodyEnabled() bool { return true }

// RequestBodyEnabled makes the client pass response body to logger.
func (l *ESLogger) ResponseBodyEnabled() bool { return true }
