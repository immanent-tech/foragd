/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package imgproxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"
	"github.com/zeebo/xxh3"
	"golang.org/x/sync/singleflight"

	"github.com/immanent-tech/go-base/client"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/providers/zyte"
	"github.com/immanent-tech/foragd/web"
)

const (
	cacheControlHeaderValue = "public, max-age=31536000, immutable"

	// maxImageSize is the largest image (in bytes) that will be fetched, served and cached.
	maxImageSize = 20 << 20
	// maxPooledBufSize is the largest buffer that is returned to the pool. Anything bigger is left for the GC so
	// one huge image doesn't pin memory forever.
	maxPooledBufSize = 1 << 20

	// fetchTimeout bounds the whole upstream fetch (including any Zyte fallback), independent of the client.
	fetchTimeout = 45 * time.Second
	// saveTimeout bounds the background cache write.
	saveTimeout = 15 * time.Second
)

var (
	bufPool = sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}

	// loadPlaceholder reads the embedded placeholder once.
	loadPlaceholder = sync.OnceValues(func() ([]byte, error) {
		return web.Files.ReadFile("files/images/placeholder.webp")
	})

	errBlockedAddress = errors.New("blocked address")
	cgnatPrefix       = netip.MustParsePrefix("100.64.0.0/10")
)

type AppConfig interface {
	GetAppName() string
	GetAppVersion() string
}

type ImageCache interface {
	GetImage(ctx context.Context, key string, buf *bytes.Buffer) error
	SaveImage(ctx context.Context, id string, data []byte) error
}

// SignatureVerifier checks the signature segment against the signed path. path is everything after the signature,
// including the leading slash (i.e. "/<options...>/<encoded url>"). Return true if the signature is valid. See
// NewSignatureVerifier for an imgproxy-compatible implementation.
type SignatureVerifier func(signature, path string) bool

// Option configures the image handler.
type Option func(*handler)

// WithSignatureVerifier enables signature verification of incoming requests. Strongly recommended when no image
// proxy prefix is configured, otherwise the handler is an open proxy.
func WithSignatureVerifier(v SignatureVerifier) Option {
	return func(h *handler) { h.verify = v }
}

type handler struct {
	cache      ImageCache
	httpClient *resty.Client
	prefix     string
	verify     SignatureVerifier
	flight     singleflight.Group
}

// HandleImage is handler that will attempt to proxy an image through the image proxy. It will fetch, store
// and retrieve the image from the cache as needed.
func HandleImage(cache ImageCache, opts ...Option) http.HandlerFunc {
	// Load config once, not on every request.
	cfg, err := loadConfig()
	if err != nil {
		slogctx.Error(context.Background(), "Load image proxy config failed.", slog.Any("error", err))
		return func(res http.ResponseWriter, _ *http.Request) {
			http.Error(res, "image proxy unavailable", http.StatusInternalServerError)
		}
	}

	h := &handler{
		cache:  cache,
		prefix: cfg.Prefix,
		// With no prefix we fetch arbitrary user-supplied URLs directly, so block internal addresses. With a
		// prefix we talk to our own imgproxy instance (which may well be on a private network).
		httpClient: newClient(cfg.Prefix == ""),
	}

	verifer, err := NewSignatureVerifier(cfg.Key, cfg.Salt)
	if err != nil {
		slogctx.Error(context.Background(), "Load image proxy config failed.", slog.Any("error", err))
		return func(res http.ResponseWriter, _ *http.Request) {
			http.Error(res, "image proxy unavailable", http.StatusInternalServerError)
		}
	}

	opts = append(opts, WithSignatureVerifier(verifer))

	for _, opt := range opts {
		opt(h)
	}
	return h.serve
}

func (h *handler) serve(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// The path is of the form /<signature>/<options...>/<encoded url>[.ext]. Tolerate a missing leading slash.
	paramStr := strings.TrimPrefix(chi.URLParam(req, "*"), "/")
	signature, rest, ok := strings.Cut(paramStr, "/")
	if !ok || signature == "" || rest == "" {
		http.Error(res, "invalid image path", http.StatusBadRequest)
		return
	}
	params := strings.Split(rest, "/")
	lastParam := params[len(params)-1]
	if lastParam == "" {
		http.Error(res, "invalid image path", http.StatusBadRequest)
		return
	}

	// imgproxy signs everything after the signature segment, including the leading slash.
	if h.verify != nil && !h.verify(signature, "/"+rest) {
		http.Error(res, "invalid signature", http.StatusForbidden)
		return
	}

	key := cacheKey(rest)
	etag := `"` + key + `"`

	// The key is content-addressed (URL + options), so a matching ETag means the client already has the image.
	if etagMatches(req.Header.Get("If-None-Match"), etag) {
		res.Header().Set("ETag", etag)
		res.Header().Set("Cache-Control", cacheControlHeaderValue)
		res.WriteHeader(http.StatusNotModified)
		return
	}

	if h.serveFromCache(res, req, key) {
		return
	}

	originalURL, err := decodeOriginalURL(lastParam)
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Decode image URL failed.", slog.Any("error", err))
		http.Error(res, "invalid image URL", http.StatusBadRequest)
		return
	}

	// Either send the image through the image proxy for processing or fetch it directly.
	fetchURL := originalURL
	if h.prefix != "" {
		fetchURL = strings.TrimRight(h.prefix, "/") + "/" + paramStr
	}

	// Collapse concurrent requests for the same image into one upstream fetch. DoChan lets each waiter bail out
	// on its own context without cancelling the shared fetch.
	ch := h.flight.DoChan(key, func() (any, error) {
		return h.fetchAndCache(ctx, key, fetchURL, originalURL)
	})

	select {
	case <-ctx.Done():
		return // Client went away; nothing to write.
	case result := <-ch:
		if result.Err != nil {
			writeError(ctx, res, result.Err)
			return
		}
		data, _ := result.Val.([]byte)
		ct, _ := sniffImageType(data) // Already validated in fetchAndCache.
		writeImage(res, req, key, ct, data)
	}
}

// serveFromCache tries to write the image from the cache. It returns true if the response was written.
func (h *handler) serveFromCache(res http.ResponseWriter, req *http.Request, key string) bool {
	buf := getBuf()
	defer putBuf(buf)

	if err := h.cache.GetImage(req.Context(), key, buf); err != nil {
		// The ImageCache interface doesn't distinguish a miss from a failure, so keep this at debug level.
		slogctx.FromCtx(req.Context()).Debug("Image not served from cache.",
			slog.String("key", key), slog.Any("error", err))
		return false
	}
	ct, ok := sniffImageType(buf.Bytes())
	if !ok {
		slogctx.FromCtx(req.Context()).Warn("Cached image is not a valid image, refetching.",
			slog.String("key", key))
		return false
	}
	writeImage(res, req, key, ct, buf.Bytes())
	return true
}

// fetchAndCache fetches the image, validates it and saves it to the cache in the background. The returned slice
// is owned by the caller(s) and must be treated as read-only.
func (h *handler) fetchAndCache(ctx context.Context, key, fetchURL, originalURL string) ([]byte, error) {
	// The fetch is shared between callers and has already been paid for, so don't tie it to one client's
	// connection. Keep context values (logger etc.).
	base := context.WithoutCancel(ctx)
	fetchCtx, cancel := context.WithTimeout(base, fetchTimeout)
	defer cancel()

	buf := getBuf()
	defer putBuf(buf)

	if err := h.fetchRemoteImage(fetchCtx, fetchURL, originalURL, buf); err != nil {
		return nil, err
	}
	data := bytes.Clone(buf.Bytes())

	if _, ok := sniffImageType(data); !ok {
		return nil, models.NewAPIError(
			http.StatusUnsupportedMediaType,
			errors.New("response is not a supported image"),
		)
	}

	go h.save(base, key, data)
	return data, nil
}

// save writes the image to the cache, detached from any request.
func (h *handler) save(base context.Context, key string, data []byte) {
	ctx, cancel := context.WithTimeout(base, saveTimeout)
	defer cancel()
	if err := h.cache.SaveImage(ctx, key, data); err != nil {
		slogctx.FromCtx(ctx).Error("Save image to cache failed.",
			slog.String("key", key), slog.Any("error", err))
	}
}

// fetchRemoteImage fetches the image directly (or via imgproxy), falling back to the Zyte proxy on a 403. The
// fallback always uses the original image URL, never the imgproxy URL.
func (h *handler) fetchRemoteImage(ctx context.Context, fetchURL, originalURL string, buf *bytes.Buffer) error {
	err := h.directFetchRemoteImage(ctx, fetchURL, buf)
	if err == nil {
		return nil
	}
	if apiErr, ok := errors.AsType[*models.APIError](err); ok && apiErr.StatusCode == http.StatusForbidden {
		buf.Reset()
		return proxyFetchRemoteImage(ctx, originalURL, buf)
	}
	return err
}

// directFetchRemoteImage fetches the image at the given url and writes it into the image buffer.
func (h *handler) directFetchRemoteImage(ctx context.Context, urlStr string, buf *bytes.Buffer) error {
	remoteURL, err := url.Parse(urlStr)
	if err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("parse URL: %w", err))
	}
	if !remoteURL.IsAbs() || (remoteURL.Scheme != "http" && remoteURL.Scheme != "https") {
		return models.NewAPIError(
			http.StatusUnprocessableEntity,
			fmt.Errorf("not an absolute http(s) URL: %q", remoteURL.Redacted()),
		)
	}

	resp, err := h.httpClient.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(urlStr)
	if err != nil {
		return models.NewAPIError(http.StatusBadGateway, fmt.Errorf("fetch image: %w", err))
	}
	// Always close the body, on every path, or the connection can't be reused.
	body := resp.RawBody()
	defer body.Close()

	if resp.IsError() {
		return models.NewAPIError(resp.StatusCode(), errors.New(resp.Status()))
	}
	if ct := resp.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		return models.NewAPIError(
			http.StatusUnsupportedMediaType,
			fmt.Errorf("unexpected content-type %q", ct),
		)
	}
	if resp.RawResponse != nil && resp.RawResponse.ContentLength > maxImageSize {
		return models.NewAPIError(http.StatusRequestEntityTooLarge, errors.New("image too large"))
	}

	// Read one byte past the limit so truncation is detected rather than silently served and cached.
	n, err := io.Copy(buf, io.LimitReader(body, maxImageSize+1))
	if err != nil {
		return models.NewAPIError(http.StatusBadGateway, fmt.Errorf("read image body: %w", err))
	}
	if n > maxImageSize {
		buf.Reset()
		return models.NewAPIError(http.StatusRequestEntityTooLarge, errors.New("image too large"))
	}
	return nil
}

// proxyFetchRemoteImage fetches the image at the given URL via a proxy and writes it into the image buffer.
func proxyFetchRemoteImage(ctx context.Context, remoteURL string, buf *bytes.Buffer) error {
	resp, err := zyte.Proxy(
		ctx,
		remoteURL,
		zyte.WithResponseBody(true),
		zyte.WithFollowRedirects(true),
		zyte.WithTag("action", "proxy_image"),
	)
	if err != nil {
		return models.NewAPIError(http.StatusBadGateway, fmt.Errorf("fetch image via proxy: %w", err))
	}
	if resp.HttpResponseBody == nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, errors.New("empty response body"))
	}

	data, err := decodeBase64(*resp.HttpResponseBody)
	if err != nil {
		return models.NewAPIError(http.StatusUnprocessableEntity, fmt.Errorf("decode base64 image: %w", err))
	}
	if len(data) > maxImageSize {
		return models.NewAPIError(http.StatusRequestEntityTooLarge, errors.New("image too large"))
	}
	buf.Write(data) // bytes.Buffer.Write never returns an error.
	return nil
}

// decodeBase64 decodes standard base64, tolerating missing padding.
func decodeBase64(s string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		if raw, rawErr := base64.RawStdEncoding.DecodeString(s); rawErr == nil {
			return raw, nil
		}
		return nil, err
	}
	return data, nil
}

// decodeOriginalURL decodes the base64url-encoded source URL from the last path segment. An optional file
// extension (e.g. ".webp") is ignored.
func decodeOriginalURL(segment string) (string, error) {
	if i := strings.IndexByte(segment, '.'); i >= 0 {
		segment = segment[:i]
	}
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return "", fmt.Errorf("decode base64 url: %w", err)
	}
	return string(decoded), nil
}

// cacheKey generates a unique key for the image and processing options. rest is the path after the signature
// (options + encoded URL), so the signature itself is excluded.
func cacheKey(rest string) string {
	h := xxh3.HashString128(rest)
	return fmt.Sprintf("%016x%016x", h.Hi, h.Lo)
}

// sniffImageType returns the content type of the data if it is a supported raster image. SVG and anything else
// is rejected.
func sniffImageType(data []byte) (string, bool) {
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		switch string(data[8:12]) {
		case "avif", "avis":
			return "image/avif", true
		}
	}
	ct := http.DetectContentType(data)
	if strings.HasPrefix(ct, "image/") {
		return ct, true
	}
	return "", false
}

// writeImage writes a successful image response. ServeContent handles Content-Length, Range and conditional
// requests.
func writeImage(res http.ResponseWriter, req *http.Request, key, contentType string, data []byte) {
	h := res.Header()
	h.Set("Content-Type", contentType)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", cacheControlHeaderValue)
	h.Set("ETag", `"`+key+`"`)
	http.ServeContent(res, req, "", time.Time{}, bytes.NewReader(data))
}

// writeError logs the error and writes a placeholder image with an appropriate status.
func writeError(ctx context.Context, res http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if apiErr, ok := errors.AsType[*models.APIError](err); ok {
		status = apiErr.StatusCode
	}
	if status < 400 || status > 599 {
		status = http.StatusBadGateway
	}
	slogctx.FromCtx(ctx).Error("Fetch remote image failed.",
		slog.Int("status", status), slog.Any("error", err))
	sendImagePlaceholder(ctx, res, status)
}

// sendImagePlaceholder writes the placeholder image with the given status. If the placeholder can't be loaded, only
// the status is written. Headers are set before WriteHeader/Write so they actually take effect.
func sendImagePlaceholder(ctx context.Context, res http.ResponseWriter, status int) {
	data, err := loadPlaceholder()
	if err != nil {
		slogctx.FromCtx(ctx).Warn("Could not load placeholder image.", slog.Any("error", err))
		res.WriteHeader(status)
		return
	}
	h := res.Header()
	h.Set("Content-Type", "image/webp")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store") // Don't let a CDN cache a transient failure as the image.
	res.WriteHeader(status)
	_, _ = res.Write(data)
}

// etagMatches reports whether an If-None-Match header value contains the given (strong) ETag.
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == etag {
			return true
		}
	}
	return false
}

func getBuf() *bytes.Buffer {
	buf, _ := bufPool.Get().(*bytes.Buffer)
	if buf == nil {
		buf = new(bytes.Buffer)
	}
	buf.Reset()
	return buf
}

func putBuf(buf *bytes.Buffer) {
	if buf.Cap() > maxPooledBufSize {
		return
	}
	bufPool.Put(buf)
}

// newClient builds an HTTP client optimised for concurrent image fetching. If blockInternal is true, connections
// to loopback, private, link-local and similar addresses are refused (SSRF protection).
func newClient(blockInternal bool) *resty.Client {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true, // required when you supply a custom transport
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   80, // match your per-host concurrency
		MaxConnsPerHost:       80, // hard cap so you don't hammer one origin
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	if blockInternal {
		// Control runs against the resolved IP on every dial, which also covers redirects and DNS rebinding.
		// An environment proxy would hide the real target from Control, so don't use one here.
		dialer.Control = denyInternalAddrs
	} else {
		transport.Proxy = http.ProxyFromEnvironment
	}

	return client.New().
		SetTransport(transport).
		SetTimeout(30*time.Second). // total per-request, including body read
		SetRedirectPolicy(resty.FlexibleRedirectPolicy(5)).
		SetHeader("Accept", "image/*").
		SetRetryCount(1).
		SetRetryWaitTime(500 * time.Millisecond).
		SetRetryMaxWaitTime(2 * time.Second).
		AddRetryCondition(shouldRetry)
}

// shouldRetry retries transient failures only. Cancelled requests, blocked addresses and unknown hosts won't
// get better on a second attempt.
func shouldRetry(r *resty.Response, err error) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, errBlockedAddress) {
			return false
		}
		if dnsErr, ok := errors.AsType[*net.DNSError](err); ok && dnsErr.IsNotFound {
			return false
		}
		return true
	}
	if r == nil {
		return false
	}
	c := r.StatusCode()
	return c == http.StatusTooManyRequests || c >= 500
}

// denyInternalAddrs is a net.Dialer.Control function that refuses non-public destinations.
func denyInternalAddrs(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	ip = ip.Unmap()
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		cgnatPrefix.Contains(ip) {
		return fmt.Errorf("%w: %s", errBlockedAddress, ip)
	}
	return nil
}
