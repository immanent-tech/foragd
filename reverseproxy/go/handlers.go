// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package reverseproxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/foragd/config"
)

var loadHTTPClient = sync.OnceValue(func() *resty.Client {
	return resty.New().
		SetHeader("User-Agent", config.AppName+"/"+config.Version).
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "gzip, deflate")

})

func handleReverseProxy(res http.ResponseWriter, req *http.Request) {
	// Extract URL parameters.
	signature := chi.URLParam(req, "signature")
	encodedURL := chi.URLParam(req, "encodedURL")

	// Don't continue if signature or URL is unset.
	if signature == "" || encodedURL == "" {
		res.WriteHeader(http.StatusBadRequest)
		http.Error(res, "bad request", http.StatusBadRequest)
		return
	}

	// Decode signature.
	messageMAC, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Could not decode signature.",
			slog.Any("error", err),
		)
		http.Error(res, "internal error", http.StatusInternalServerError)
		return
	}

	mac := hmac.New(sha256.New, []byte(cfg.Key))
	mac.Write([]byte(cfg.Salt))
	mac.Write([]byte(encodedURL))
	expectedMAC := mac.Sum(nil)

	// Check signature.
	if !hmac.Equal(messageMAC, expectedMAC) {
		res.WriteHeader(http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Signature does not match message.",
			slog.Any("error", err),
		)
		http.Error(res, "internal error", http.StatusInternalServerError)
		return
	}

	// Decode URL.
	url, err := base64.RawURLEncoding.DecodeString(encodedURL)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Could not decode url.",
			slog.Any("error", err),
		)
		http.Error(res, "internal error", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers.
	res.Header().Set("Access-Control-Allow-Origin", "*")
	res.Header().Set("X-Proxied-By", "Cloudflare-Worker")
	res.Header().Del("Set-Cookie")

	client := loadHTTPClient()

	// Proxy the URL.
	resp, err := client.R().SetDoNotParseResponse(true).Get(string(url))
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		slogctx.FromCtx(req.Context()).Error("Proxy URL failed.",
			slog.String("url", string(url)),
			slog.Any("error", err),
		)
		return
	}
	if resp.IsError() {
		res.WriteHeader(resp.StatusCode())
		slogctx.FromCtx(req.Context()).Error("Image proxy returned error status code.",
			slog.String("url", string(url)),
			slog.Any("error", resp.Status()),
		)
		return
	}

	defer resp.RawBody().Close()

	body, err := io.ReadAll(resp.RawBody())

	res.WriteHeader(resp.StatusCode())
	res.Write(body)
}
