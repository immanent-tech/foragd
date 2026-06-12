// Package etag implements middleware for handling the ETag header in responses. It is modified from the original
// package github.com/go-http-utils/etag to use xxHash instead of sha1.
package etag

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
)

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

func (hw *hashWriter) WriteHeader(status int) {
	hw.status = status
}

func (hw *hashWriter) Write(data []byte) (int, error) {
	if hw.status == 0 {
		hw.status = http.StatusOK
	}
	// bytes.Buffer.Write(b) always return (len(b), nil), so just
	// ignore the return values.
	hw.buf.Write(data)

	l, err := hw.hash.Write(data)
	hw.len += l
	if err != nil {
		return l, fmt.Errorf("write data: %w", err)
	}
	return l, nil
}

func (hw *hashWriter) Reset() {
	hw.hash = xxh3.New()
	hw.buf.Reset()
}

// Handler wraps the http.Handler h with ETag support.
func Handler(next http.Handler, weak bool) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		hw, ok := hwPool.Get().(*hashWriter)
		if !ok {
			slogctx.FromCtx(req.Context()).Error("Could not generate ETag.",
				slog.String("error", "could not get hashWriter buffer"))
			next.ServeHTTP(res, req)
			return
		}
		hw.rw = res
		defer func() {
			hw.Reset()
			hwPool.Put(hw)
		}()
		next.ServeHTTP(hw, req)

		resHeader := res.Header()

		if resHeader.Get("ETag") != "" ||
			hw.status < 200 || hw.status >= 300 ||
			hw.status == http.StatusNoContent ||
			hw.buf.Len() == 0 {
			res.WriteHeader(hw.status)
			res.Write(hw.buf.Bytes())
			return
		}

		etag := fmt.Sprintf("%v-%v", strconv.Itoa(hw.len),
			hex.EncodeToString(hw.hash.Sum(nil)))

		if weak {
			etag = "W/" + etag
		}

		resHeader.Set("ETag", etag)

		if isFresh(req.Header, resHeader) {
			res.WriteHeader(http.StatusNotModified)
			res.Write(nil)
		} else {
			res.WriteHeader(hw.status)
			res.Write(hw.buf.Bytes())
		}
	})
}

func isFresh(reqHeader http.Header, resHeader http.Header) bool {
	ifNoneMatch := reqHeader.Get("If-None-Match")
	ifModifiedSince := reqHeader.Get("If-Modified-Since")
	cacheControl := reqHeader.Get("Cache-Control")
	etag := resHeader.Get("ETag")
	lastModified := resHeader.Get("Last-Modified")

	if ifNoneMatch == "" && ifModifiedSince == "" {
		return false
	}
	if strings.Contains(cacheControl, "no-cache") {
		return false
	}

	if etag != "" && ifNoneMatch != "" {
		return checkEtagNoneMatch(trimTags(strings.Split(ifNoneMatch, ",")), etag)
	}
	if lastModified != "" && ifModifiedSince != "" {
		return checkModifedMatch(lastModified, ifModifiedSince)
	}
	return false
}

func trimTags(tags []string) []string {
	trimedTags := make([]string, len(tags))

	for i, tag := range tags {
		trimedTags[i] = strings.TrimSpace(tag)
	}

	return trimedTags
}

func checkEtagNoneMatch(etagsToNoneMatch []string, etag string) bool {
	for _, etagToNoneMatch := range etagsToNoneMatch {
		if etagToNoneMatch == "*" || etagToNoneMatch == etag || etagToNoneMatch == "W/"+etag {
			return true
		}
	}

	return false
}

func checkModifedMatch(lastModified, ifModifiedSince string) bool {
	if lm, ims, ok := parseTimePairs(lastModified, ifModifiedSince); ok {
		return lm.Before(ims)
	}

	return false
}

func parseTimePairs(s1, s2 string) (t1_1 time.Time, t2_1 time.Time, ok bool) {
	if t1, err := time.Parse(http.TimeFormat, s1); err == nil {
		if t2, err := time.Parse(http.TimeFormat, s2); err == nil {
			return t1, t2, true
		}
	}

	return t1_1, t2_1, false
}

var hwPool = sync.Pool{
	New: func() any {
		return &hashWriter{hash: xxh3.New(), buf: bytes.NewBuffer(nil)}
	},
}
