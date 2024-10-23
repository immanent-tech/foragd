// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package elastic

import (
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

	// Count number of bytes in request and response.
	//
	if req != nil && req.Body != nil && req.Body != http.NoBody {
		nReq, _ = io.Copy(io.Discard, req.Body) //nolint:errcheck
	}

	if res != nil && res.Body != nil && res.Body != http.NoBody {
		nRes, _ = io.Copy(io.Discard, res.Body) //nolint:errcheck
	}

	// Log event.
	//
	l.Info("Request",
		slog.String("server", req.URL.Host),
		slog.String("path", req.URL.Path),
		slog.String("method", req.Method),
		slog.Int("status_code", res.StatusCode),
		slog.Duration("duration", dur),
		slog.Int64("req_bytes", nReq),
		slog.Int64("res_bytes", nRes))

	return nil
}

// RequestBodyEnabled makes the client pass request body to logger.
//
//revive:disable:unused-receiver
func (l *ESLogger) RequestBodyEnabled() bool { return true }

// RequestBodyEnabled makes the client pass response body to logger.
//
//revive:disable:unused-receiver
func (l *ESLogger) ResponseBodyEnabled() bool { return true }
