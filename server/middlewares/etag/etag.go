// Package etag implements middleware for handling the ETag header in responses. It is modified from the original
// package github.com/go-http-utils/etag to use xxHash instead of sha1.
package etag

import (
	"bytes"
	"fmt"
	"hash"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
)

// hashWriter buffers the downstream response so we can compute an ETag before flushing to the real ResponseWriter.
type hashWriter struct {
	rw     http.ResponseWriter
	hash   hash.Hash
	buf    *bytes.Buffer
	len    int
	status int
}

func (hw *hashWriter) Header() http.Header {
	return hw.rw.Header()
}

// WriteHeader captures the status code without forwarding it yet. The real WriteHeader call is deferred until after the
// ETag logic in Handler.
func (hw *hashWriter) WriteHeader(status int) {
	hw.status = status
}

// Write buffers response bytes. No streaming hash is maintained here; the hash is computed in a single pass over the
// buffer after the handler returns, which lets xxh3 use its fastest code path and avoids per-Write allocations.
func (hw *hashWriter) Write(data []byte) (int, error) {
	if hw.status == 0 {
		hw.status = http.StatusOK
	}
	n, err := hw.buf.Write(data)
	if err != nil {
		return n, fmt.Errorf("write data: %w", err)
	}
	hw.len += n
	return n, nil
}

// reset clears the writer for reuse via the pool. The rw field is also cleared to avoid retaining a reference to the
// previous request's ResponseWriter.
func (hw *hashWriter) reset() {
	hw.rw = nil
	hw.buf.Reset()
	hw.len = 0
	hw.status = 0
}

// Handler wraps the http.Handler h with ETag support.
func Handler(next http.Handler, weak bool) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		hw, ok := hwPool.Get().(*hashWriter)
		if !ok {
			slogctx.FromCtx(req.Context()).Error("Could not generate ETag.",
				slog.String("error", "could not get hashWriter from pool"))
			next.ServeHTTP(res, req)
			return
		}

		hw.rw = res
		defer func() {
			hw.reset()
			hwPool.Put(hw)
		}()

		next.ServeHTTP(hw, req)

		resHeader := res.Header()

		// Skip ETag generation when:
		//   - the handler already set one
		//   - the status is outside the 2xx range (or 204 No Content)
		//   - the body is empty
		if resHeader.Get("ETag") != "" ||
			hw.status < 200 || hw.status >= 300 ||
			hw.status == http.StatusNoContent ||
			hw.buf.Len() == 0 {
			res.WriteHeader(hw.status)
			res.Write(hw.buf.Bytes()) //nolint:errcheck
			return
		}

		// Single-pass hash over the complete buffer — faster than streaming and
		// avoids maintaining a hash.Hash in the struct.
		body := hw.buf.Bytes()
		sum := xxh3.Hash128(body)

		// ETags must be enclosed in double-quotes per RFC 9110 §8.8.3.
		etag := fmt.Sprintf("\"%d-%016x%016x\"", hw.len, sum.Hi, sum.Lo)
		if weak {
			etag = "W/" + etag
		}

		resHeader.Set("ETag", etag)

		if isFresh(req, resHeader) {
			res.WriteHeader(http.StatusNotModified)
			return
		}

		res.WriteHeader(hw.status)
		res.Write(hw.buf.Bytes())
	})
}

// isFresh reports whether the request's cache validators indicate the client
// already holds a fresh copy of the resource.
//
// Evaluation order follows RFC 9110 §13.1:
//  1. If-None-Match is checked first (ETag-based).
//  2. If-Modified-Since is used as a fallback when no ETag comparison applies.
//
// Requests with Cache-Control: no-cache are never considered fresh.
// If-None-Match with * is restricted to safe methods (GET, HEAD) to avoid
// incorrectly satisfying conditional writes.
func isFresh(req *http.Request, resHeader http.Header) bool {
	reqHeader := req.Header

	ifNoneMatch := reqHeader.Get("If-None-Match")
	ifModifiedSince := reqHeader.Get("If-Modified-Since")

	// Nothing to validate against.
	if ifNoneMatch == "" && ifModifiedSince == "" {
		return false
	}

	// Cache-Control: no-cache forces revalidation regardless of ETags.
	if strings.Contains(reqHeader.Get("Cache-Control"), "no-cache") {
		return false
	}

	if ifNoneMatch != "" {
		etag := resHeader.Get("ETag")
		if etag != "" {
			return checkEtagNoneMatch(trimTags(strings.Split(ifNoneMatch, ",")), etag, req.Method)
		}
	}

	// ETag check was inconclusive; fall back to date-based validation.
	if ifModifiedSince != "" {
		lastModified := resHeader.Get("Last-Modified")
		if lastModified != "" {
			return checkModifiedMatch(lastModified, ifModifiedSince)
		}
	}

	return false
}

// checkEtagNoneMatch returns true when any of the client-supplied ETags matches
// the server ETag using weak comparison (RFC 9110 §8.8.3.2), i.e. the W/
// prefix is stripped before comparing both sides.
//
// The wildcard "*" is only honoured on safe methods (GET, HEAD) because on
// unsafe methods it carries "match if any current representation exists"
// semantics that should not result in a 304.
func checkEtagNoneMatch(candidates []string, etag, method string) bool {
	for _, c := range candidates {
		if c == "*" {
			// Only safe for GET/HEAD — unsafe methods must not be short-circuited
			// to 304 by a wildcard If-None-Match.
			return method == http.MethodGet || method == http.MethodHead
		}
		// Weak comparison: strip W/ from both sides before comparing.
		if strings.TrimPrefix(c, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

// checkModifiedMatch returns true when the resource has not been modified since
// the client's cached copy, i.e. Last-Modified <= If-Modified-Since.
// A resource modified at exactly the same second as the client's copy is
// considered unmodified (not strictly before).
func checkModifiedMatch(lastModified, ifModifiedSince string) bool {
	lm, ims, ok := parseTimePair(lastModified, ifModifiedSince)
	if !ok {
		return false
	}
	// !After ≡ Before || Equal — equal timestamps mean "not modified".
	return !lm.After(ims)
}

// parseTimePair parses two HTTP-date strings (RFC 9110 §5.6.7).
// Returns ok=false if either string fails to parse.
func parseTimePair(s1, s2 string) (t1, t2 time.Time, ok bool) {
	var err error
	if t1, err = time.Parse(http.TimeFormat, s1); err != nil {
		return
	}
	if t2, err = time.Parse(http.TimeFormat, s2); err != nil {
		return
	}
	ok = true
	return
}

// trimTags strips surrounding whitespace from each ETag token. The HTTP spec
// allows optional whitespace around the comma-separated list items.
func trimTags(tags []string) []string {
	trimmed := make([]string, len(tags))
	for i, t := range tags {
		trimmed[i] = strings.TrimSpace(t)
	}
	return trimmed
}

// hwPool recycles hashWriter instances to avoid per-request allocations.
// Buffers are pre-allocated to 4 KiB to cover most typical response sizes
// without re-allocating.
var hwPool = sync.Pool{
	New: func() any {
		return &hashWriter{
			buf: bytes.NewBuffer(make([]byte, 0, 4096)),
		}
	},
}
