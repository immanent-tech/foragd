// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package service

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/go-resty/resty/v2"
	"github.com/immanent-tech/go-base/client"
	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/pkg/htmlx"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("github.com/immanent-tech/foragd/service")

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func extractMetadataFromHTML(
	sourceURL *url.URL,
	source []byte,
) (*htmlx.OpenGraph, *readability.Article, error) {
	var errs []error
	// Extract any Opengraph data.
	opengraphData, err := htmlx.DecodeOpengraph(bytes.NewReader(source))
	if err != nil {
		errs = append(errs, err)
	}
	// Extract readability data.
	readabilityData, err := readability.FromReader(bytes.NewReader(source), sourceURL)
	if err != nil {
		errs = append(errs, err)
	}

	return opengraphData, &readabilityData, errors.Join(errs...)
}

var newHTTPClient = sync.OnceValue(func() *resty.Client {
	var userAgent string
	if appCfg, err := config.LoadAppConfig(); err != nil {
		userAgent = "Foragd/Unknown (+https://foragd.app/policies/bot)"
	} else {
		userAgent = appCfg.GetAppName() + "/" + appCfg.GetAppVersion() + " (+https://foragd.app/policies/bot)"

	}
	httpClient := client.New().
		SetTransport(&http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true, // required when you supply a custom transport
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   32, // match your per-host concurrency
			MaxConnsPerHost:       32, // hard cap so you don't hammer one origin
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		}).
		SetHeader("User-Agent", userAgent).
		SetRedirectPolicy(resty.FlexibleRedirectPolicy(5)).
		SetRetryCount(3).
		SetRetryWaitTime(500 * time.Millisecond).
		SetRetryMaxWaitTime(5 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			c := r.StatusCode()
			return c == 429 || c >= 500
		})
	return httpClient
})
