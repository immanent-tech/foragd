/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package imgproxy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
)

const (
	testKeyHex  = "736563726574" // "secret"
	testSaltHex = "73616c74"     // "salt"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// fakeCache is an in-memory ImageCache. Saves happen in a background goroutine in the handler, so tests wait on
// the saved channel.
type fakeCache struct {
	mu    sync.Mutex
	data  map[string][]byte
	gets  int
	saves int
	saved chan string
}

func newFakeCache() *fakeCache {
	return &fakeCache{data: make(map[string][]byte), saved: make(chan string, 16)}
}

func (c *fakeCache) GetImage(_ context.Context, key string, buf *bytes.Buffer) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gets++
	d, ok := c.data[key]
	if !ok {
		return errors.New("not found")
	}
	buf.Write(d)
	return nil
}

func (c *fakeCache) SaveImage(_ context.Context, id string, data []byte) error {
	c.mu.Lock()
	c.data[id] = bytes.Clone(data)
	c.saves++
	c.mu.Unlock()
	select {
	case c.saved <- id:
	default:
	}
	return nil
}

func (c *fakeCache) set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = data
}

func (c *fakeCache) counts() (gets, saves int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gets, c.saves
}

func (c *fakeCache) waitSaved(t *testing.T) {
	t.Helper()
	select {
	case <-c.saved:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cache save")
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return b.Bytes()
}

// newUpstream starts a server that always replies with the given status, content type and body, and counts hits.
func newUpstream(t *testing.T, status int, contentType string, body []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// restPath is the part of the URL after the signature: <options>/<encoded url>.png
func restPath(options, rawURL string) string {
	return options + "/" + base64.RawURLEncoding.EncodeToString([]byte(rawURL)) + ".png"
}

// imgPath builds a request path in the /<signature>/<rest> form.
func imgPath(sig, options, rawURL string) string {
	return "/" + sig + "/" + restPath(options, rawURL)
}

func signPath(path string) string {
	key, _ := hex.DecodeString(testKeyHex)
	salt, _ := hex.DecodeString(testSaltHex)
	mac := hmac.New(sha256.New, key)
	mac.Write(salt)
	mac.Write([]byte(path))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// newTestHandler uses a non-blocking client so tests can talk to httptest servers on loopback.
func newTestHandler(cache ImageCache, verify SignatureVerifier) *handler {
	return &handler{cache: cache, httpClient: newClient(false), verify: verify}
}

// doRequest calls the handler with the chi wildcard param set to path (as it would be in the router).
func doRequest(h *handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/img"+path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", path)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.serve(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// Signature verification
// ---------------------------------------------------------------------------

func TestNewSignatureVerifier_Errors(t *testing.T) {
	tests := []struct {
		name, key, salt string
	}{
		{"bad key hex", "zz", testSaltHex},
		{"empty key", "", testSaltHex},
		{"bad salt hex", testKeyHex, "zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewSignatureVerifier(tt.key, tt.salt); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}

	if _, err := NewSignatureVerifier(testKeyHex, ""); err != nil {
		t.Fatalf("empty salt should be allowed: %v", err)
	}
}

func TestSignatureVerifier(t *testing.T) {
	verify, err := NewSignatureVerifier(testKeyHex, testSaltHex)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := NewSignatureVerifier("6f74686572", testSaltHex) // "other"
	if err != nil {
		t.Fatal(err)
	}

	path := "/" + restPath("rs:fit:300:300", "https://example.com/a.jpg")
	full := signPath(path)
	raw, err := base64.RawURLEncoding.DecodeString(full)
	if err != nil {
		t.Fatal(err)
	}
	truncated := base64.RawURLEncoding.EncodeToString(raw[:16])
	tooShort := base64.RawURLEncoding.EncodeToString(raw[:4])

	tests := []struct {
		name     string
		verifier SignatureVerifier
		sig      string
		path     string
		want     bool
	}{
		{"valid full signature", verify, full, path, true},
		{"valid truncated signature", verify, truncated, path, true},
		{"signature too short", verify, tooShort, path, false},
		{"signature too long", verify, full + "AAAA", path, false},
		{"tampered options", verify, full, strings.Replace(path, "300", "999", 1), false},
		{"path without leading slash", verify, full, strings.TrimPrefix(path, "/"), false},
		{"signature for different path", verify, signPath("/other"), path, false},
		{"insecure placeholder", verify, "insecure", path, false},
		{"empty signature", verify, "", path, false},
		{"not base64", verify, "!!!!", path, false},
		{"signed with a different key", otherKey, full, path, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.verifier(tt.sig, tt.path); got != tt.want {
				t.Fatalf("verify(%q, %q) = %v, want %v", tt.sig, tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pure helpers
// ---------------------------------------------------------------------------

func TestCacheKey(t *testing.T) {
	a := cacheKey("rs:fit:100:100/abc.png")
	if a != cacheKey("rs:fit:100:100/abc.png") {
		t.Error("cacheKey is not deterministic")
	}
	if a == cacheKey("rs:fit:200:200/abc.png") {
		t.Error("different options produced the same key")
	}
	if a == cacheKey("rs:fit:100:100/abd.png") {
		t.Error("different URL produced the same key")
	}
	if len(a) != 32 {
		t.Errorf("key length = %d, want 32 hex chars", len(a))
	}
}

func TestEtagMatches(t *testing.T) {
	etag := `"abc"`
	tests := []struct {
		header string
		want   bool
	}{
		{``, false},
		{`"abc"`, true},
		{`W/"abc"`, true},
		{`"nope", "abc"`, true},
		{`"nope"`, false},
		{`abc`, false}, // unquoted
	}
	for _, tt := range tests {
		if got := etagMatches(tt.header, etag); got != tt.want {
			t.Errorf("etagMatches(%q) = %v, want %v", tt.header, got, tt.want)
		}
	}
}

func TestSniffImageType(t *testing.T) {
	avif := []byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f', 0, 0, 0, 0}
	avis := []byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p', 'a', 'v', 'i', 's', 0, 0, 0, 0}
	heic := []byte{0, 0, 0, 0x20, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c', 0, 0, 0, 0}

	tests := []struct {
		name   string
		data   []byte
		wantCT string
		wantOK bool
	}{
		{"png", testPNG(t), "image/png", true},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F'}, "image/jpeg", true},
		{"gif", []byte("GIF89a\x01\x00\x01\x00"), "image/gif", true},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp", true},
		{"avif", avif, "image/avif", true},
		{"avif sequence", avis, "image/avif", true},
		{"heic unsupported", heic, "", false},
		{"svg rejected", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), "", false},
		{"xml svg rejected", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`), "", false},
		{"html rejected", []byte(`<!DOCTYPE html><html></html>`), "", false},
		{"empty", nil, "", false},
		{"short garbage", []byte{1, 2, 3}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct, ok := sniffImageType(tt.data)
			if ok != tt.wantOK || ct != tt.wantCT {
				t.Fatalf("sniffImageType = (%q, %v), want (%q, %v)", ct, ok, tt.wantCT, tt.wantOK)
			}
		})
	}
}

func TestDecodeOriginalURL(t *testing.T) {
	const raw = "https://example.com/a b.jpg?x=1&y=2"
	enc := base64.RawURLEncoding.EncodeToString([]byte(raw))

	for _, seg := range []string{enc, enc + ".webp", enc + ".png"} {
		got, err := decodeOriginalURL(seg)
		if err != nil || got != raw {
			t.Errorf("decodeOriginalURL(%q) = (%q, %v), want %q", seg, got, err, raw)
		}
	}
	if _, err := decodeOriginalURL("!!!not-base64!!!"); err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecodeBase64(t *testing.T) {
	data := []byte("hello image")
	padded := base64.StdEncoding.EncodeToString(data)
	unpadded := base64.RawStdEncoding.EncodeToString(data)

	for _, in := range []string{padded, unpadded} {
		got, err := decodeBase64(in)
		if err != nil || !bytes.Equal(got, data) {
			t.Errorf("decodeBase64(%q) = (%q, %v)", in, got, err)
		}
	}
	if _, err := decodeBase64("***"); err == nil {
		t.Error("expected error for invalid input")
	}
}

// ---------------------------------------------------------------------------
// SSRF protection and retry policy
// ---------------------------------------------------------------------------

func TestDenyInternalAddrs(t *testing.T) {
	tests := []struct {
		addr    string
		blocked bool
	}{
		{"127.0.0.1:80", true},
		{"[::1]:80", true},
		{"[::ffff:127.0.0.1]:80", true}, // IPv4-mapped loopback
		{"10.0.0.1:80", true},
		{"172.16.5.4:443", true},
		{"192.168.1.1:443", true},
		{"169.254.169.254:80", true}, // cloud metadata
		{"100.64.0.1:80", true},      // CGNAT
		{"0.0.0.0:80", true},
		{"224.0.0.1:80", true},
		{"[fe80::1]:80", true},
		{"[fd00::1]:80", true},
		{"8.8.8.8:53", false},
		{"[2606:4700:4700::1111]:443", false},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			err := denyInternalAddrs("tcp", tt.addr, nil)
			if tt.blocked && !errors.Is(err, errBlockedAddress) {
				t.Fatalf("expected errBlockedAddress, got %v", err)
			}
			if !tt.blocked && err != nil {
				t.Fatalf("expected allowed, got %v", err)
			}
		})
	}
}

func TestBlockingClientRejectsLoopback(t *testing.T) {
	srv, hits := newUpstream(t, http.StatusOK, "image/png", testPNG(t))

	_, err := newClient(true).R().Get(srv.URL)
	if !errors.Is(err, errBlockedAddress) {
		t.Fatalf("expected errBlockedAddress, got %v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("upstream was contacted %d times", hits.Load())
	}
}

func TestShouldRetry(t *testing.T) {
	resp := func(code int) *resty.Response {
		return &resty.Response{RawResponse: &http.Response{StatusCode: code}}
	}
	tests := []struct {
		name string
		resp *resty.Response
		err  error
		want bool
	}{
		{"generic error", nil, errors.New("boom"), true},
		{"context canceled", nil, context.Canceled, false},
		{"deadline exceeded", nil, fmt.Errorf("wrapped: %w", context.DeadlineExceeded), false},
		{"blocked address", nil, fmt.Errorf("dial: %w", errBlockedAddress), false},
		{"dns not found", nil, fmt.Errorf("lookup: %w", &net.DNSError{IsNotFound: true}), false},
		{"dns temporary", nil, &net.DNSError{IsTemporary: true}, true},
		{"nil response no error", nil, nil, false},
		{"429", resp(429), nil, true},
		{"500", resp(500), nil, true},
		{"503", resp(503), nil, true},
		{"404", resp(404), nil, false},
		{"200", resp(200), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetry(tt.resp, tt.err); got != tt.want {
				t.Fatalf("shouldRetry = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Handler
// ---------------------------------------------------------------------------

func TestServe_CacheMissThenHit(t *testing.T) {
	img := testPNG(t)
	srv, hits := newUpstream(t, http.StatusOK, "image/png", img)
	cache := newFakeCache()
	h := newTestHandler(cache, nil)
	path := imgPath("sig", "rs:fit:100:100", srv.URL+"/a.png")

	rec := doRequest(h, path, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), img) {
		t.Error("body does not match upstream image")
	}
	hdr := rec.Header()
	if got := hdr.Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := hdr.Get("Cache-Control"); got != cacheControlHeaderValue {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := hdr.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if etag := hdr.Get("ETag"); !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag = %q, want quoted value", etag)
	}

	// The save is asynchronous; wait for it, then the second request must come from the cache.
	cache.waitSaved(t)
	rec2 := doRequest(h, path, nil)
	if rec2.Code != http.StatusOK || !bytes.Equal(rec2.Body.Bytes(), img) {
		t.Fatalf("cached response: status=%d, body match=%v", rec2.Code, bytes.Equal(rec2.Body.Bytes(), img))
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
}

func TestServe_NotModified(t *testing.T) {
	srv, hits := newUpstream(t, http.StatusOK, "image/png", testPNG(t))
	cache := newFakeCache()
	h := newTestHandler(cache, nil)
	path := imgPath("sig", "rs:fit:100:100", srv.URL+"/a.png")

	first := doRequest(h, path, nil)
	etag := first.Header().Get("ETag")
	cache.waitSaved(t)
	getsBefore, _ := cache.counts()

	rec := doRequest(h, path, map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Error("304 response has a body")
	}
	if got := rec.Header().Get("Cache-Control"); got != cacheControlHeaderValue {
		t.Errorf("Cache-Control = %q", got)
	}
	if getsAfter, _ := cache.counts(); getsAfter != getsBefore {
		t.Error("cache was consulted for a conditional request that matched")
	}
	if hits.Load() != 1 {
		t.Errorf("upstream hits = %d, want 1", hits.Load())
	}
}

func TestServe_BadPaths(t *testing.T) {
	h := newTestHandler(newFakeCache(), nil)
	for _, path := range []string{
		"",
		"/",
		"sig",
		"/sig",
		"/sig/",
		"/sig/opts/",
		"/sig/opts/!!!not-base64",
	} {
		t.Run(path, func(t *testing.T) {
			if rec := doRequest(h, path, nil); rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestServe_PathWithoutLeadingSlash(t *testing.T) {
	srv, _ := newUpstream(t, http.StatusOK, "image/png", testPNG(t))
	h := newTestHandler(newFakeCache(), nil)

	path := strings.TrimPrefix(imgPath("sig", "rs:fit:100:100", srv.URL+"/a.png"), "/")
	if rec := doRequest(h, path, nil); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestServe_SignatureVerification(t *testing.T) {
	srv, hits := newUpstream(t, http.StatusOK, "image/png", testPNG(t))
	verify, err := NewSignatureVerifier(testKeyHex, testSaltHex)
	if err != nil {
		t.Fatal(err)
	}
	h := newTestHandler(newFakeCache(), verify)
	rest := restPath("rs:fit:100:100", srv.URL+"/a.png")

	t.Run("invalid signature is rejected before any fetch", func(t *testing.T) {
		rec := doRequest(h, "/"+signPath("/something-else")+"/"+rest, nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		if hits.Load() != 0 {
			t.Fatalf("upstream hits = %d, want 0", hits.Load())
		}
	})

	t.Run("insecure placeholder is rejected", func(t *testing.T) {
		if rec := doRequest(h, "/insecure/"+rest, nil); rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("valid signature is accepted", func(t *testing.T) {
		rec := doRequest(h, "/"+signPath("/"+rest)+"/"+rest, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})
}

func TestServe_UpstreamErrors(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		contentType string
		body        []byte
		wantStatus  int
	}{
		{"not found", http.StatusNotFound, "image/png", []byte("nope"), http.StatusNotFound},
		{"wrong content type", http.StatusOK, "text/html", []byte("<html></html>"), http.StatusUnsupportedMediaType},
		{
			"image header but html body",
			http.StatusOK,
			"image/png",
			[]byte("<html></html>"),
			http.StatusUnsupportedMediaType,
		},
		{
			"svg",
			http.StatusOK,
			"image/svg+xml",
			[]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
			http.StatusUnsupportedMediaType,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newUpstream(t, tt.status, tt.contentType, tt.body)
			cache := newFakeCache()
			h := newTestHandler(cache, nil)

			rec := doRequest(h, imgPath("sig", "rs:fit:100:100", srv.URL+"/a"), nil)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			if _, saves := cache.counts(); saves != 0 {
				t.Errorf("failed fetch was cached (%d saves)", saves)
			}
		})
	}
}

func TestServe_TooLarge(t *testing.T) {
	t.Run("declared content-length", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(maxImageSize+1))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("short"))
		}))
		t.Cleanup(srv.Close)

		cache := newFakeCache()
		rec := doRequest(newTestHandler(cache, nil), imgPath("sig", "o", srv.URL+"/big.png"), nil)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", rec.Code)
		}
		if _, saves := cache.counts(); saves != 0 {
			t.Error("oversized image was cached")
		}
	})

	t.Run("streamed body without content-length", func(t *testing.T) {
		if testing.Short() {
			t.Skip("allocates >20MB")
		}
		img := testPNG(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(img)
			chunk := make([]byte, 1<<20)
			for range 21 {
				_, _ = w.Write(chunk)
				w.(http.Flusher).Flush() // forces chunked encoding, so no Content-Length
			}
		}))
		t.Cleanup(srv.Close)

		cache := newFakeCache()
		rec := doRequest(newTestHandler(cache, nil), imgPath("sig", "o", srv.URL+"/big.png"), nil)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413 (truncated image must not be served)", rec.Code)
		}
		if _, saves := cache.counts(); saves != 0 {
			t.Error("truncated image was cached")
		}
	})
}

func TestServe_CorruptCacheEntryIsRefetched(t *testing.T) {
	img := testPNG(t)
	srv, hits := newUpstream(t, http.StatusOK, "image/png", img)
	cache := newFakeCache()
	h := newTestHandler(cache, nil)

	rawURL := srv.URL + "/a.png"
	cache.set(cacheKey(restPath("rs:fit:100:100", rawURL)), []byte("<html>not an image</html>"))

	rec := doRequest(h, imgPath("sig", "rs:fit:100:100", rawURL), nil)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), img) {
		t.Fatalf("status = %d, want 200 with upstream image", rec.Code)
	}
	if hits.Load() != 1 {
		t.Fatalf("upstream hits = %d, want 1", hits.Load())
	}
}

// Concurrent requests for the same uncached image must trigger a single upstream fetch. This test relies on a
// short sleep so all requests join the in-flight call before it completes.
func TestServe_Singleflight(t *testing.T) {
	img := testPNG(t)
	release := make(chan struct{})
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		<-release
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(img)
	}))
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(srv.Close)
	t.Cleanup(unblock) // runs first (LIFO), so srv.Close can't hang on a blocked handler

	h := newTestHandler(newFakeCache(), nil)
	path := imgPath("sig", "rs:fit:100:100", srv.URL+"/a.png")

	const n = 10
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = doRequest(h, path, nil).Code
		}()
	}
	time.Sleep(150 * time.Millisecond)
	unblock()
	wg.Wait()

	for i, c := range codes {
		if c != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i, c)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1", got)
	}
}

func TestServe_PrefixMode(t *testing.T) {
	var mu sync.Mutex
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNG(t))
	}))
	t.Cleanup(srv.Close)

	h := newTestHandler(newFakeCache(), nil)
	h.prefix = srv.URL + "/imgproxy/" // trailing slash must not produce a double slash

	rest := restPath("rs:fit:100:100", "https://example.com/a.png")
	rec := doRequest(h, "/sig/"+rest, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := "/imgproxy/sig/" + rest; gotPath != want {
		t.Fatalf("imgproxy received path %q, want %q", gotPath, want)
	}
}

func TestServe_BlocksInternalAddresses(t *testing.T) {
	srv, hits := newUpstream(t, http.StatusOK, "image/png", testPNG(t))
	h := &handler{cache: newFakeCache(), httpClient: newClient(true)} // blocking client, as in no-prefix mode

	rec := doRequest(h, imgPath("sig", "o", srv.URL+"/a.png"), nil) // srv is on 127.0.0.1
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if hits.Load() != 0 {
		t.Fatalf("loopback server was contacted %d times", hits.Load())
	}
}

func TestServe_InvalidSchemeRejected(t *testing.T) {
	h := newTestHandler(newFakeCache(), nil)
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/a.png", "/relative/path.png"} {
		t.Run(raw, func(t *testing.T) {
			rec := doRequest(h, imgPath("sig", "o", raw), nil)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422", rec.Code)
			}
		})
	}
}
