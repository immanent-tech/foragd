// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/a-h/templ"
	"github.com/go-resty/resty/v2"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-syndication/linter"

	"github.com/immanent-tech/foragd/models"
	"github.com/immanent-tech/foragd/web/templates"
)

type Linter struct {
	pageMetadata
}

func (p *Linter) FullResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(
		templates.CreatePage(
			templates.Linter(),
			templates.WithPageTitle(p.Title),
			templates.WithPageDescription(p.Description),
			templates.WithCanonicalLink(p.CanonicalLink()),
			templates.WithOpenGraphMetadata(p.OpengraphData()),
			templates.WithJSONLDSchema(
				generateSiteJSONLD(p.baseURL),
				p.JSONLD(),
			),
		),
	).ServeHTTP(res, req)
}

type LinterResponse struct {
	Results map[string][]linter.Result
}

func (p *LinterResponse) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.LinterResults(p.Results)).ServeHTTP(res, req)
}

type LinterError struct {
	msg *models.UserMessage
}

func (p *LinterError) PartialResponse(res http.ResponseWriter, req *http.Request) {
	templ.Handler(templates.LinterError(p.msg)).ServeHTTP(res, req)
}

func HandleLinter(appCfg AppConfig, httpClient *resty.Client) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		switch fetchErr := models.NewErrorMessage(
			"Unable to find feed at provided URL",
			"No feed details could be fetched from the given URL. This could be a temporary error.",
		); req.Method {
		case http.MethodGet:
			RenderExternalPage(&Linter{
				Title: templates.PageTitle{
					Summary:     "Free RSS Feed Linter",
					Description: "Lint Any Website's Feed to check for validation errors and recommendations",
				},
				Description: "Foragd's free feed RSS linter instantly shows whether a site's feed passes validation and contains recommended values to ensure maximum compatibility",
				Path:        "/linter",
				ImagePath:   "/content/logo-vertical-light.webp",
				baseURL:     appCfg.GetBaseURL(),
			}).ServeHTTP(res, req)
		case http.MethodPost:
			feedData, err := fetchFeedData(httpClient, req.FormValue("url"))
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed to parse feed.",
					slog.Any("error", err),
				)
				RenderPartial(&LinterError{
					msg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}

			results, err := linter.Lint(feedData)
			if err != nil {
				slogctx.FromCtx(req.Context()).Warn("Linter failed.",
					slog.Any("error", err),
				)
				RenderPartial(&LinterError{
					msg: fetchErr,
				}).ServeHTTP(res, req)
				return
			}
			RenderPartial(&LinterResponse{Results: results}).ServeHTTP(res, req)
		}
	}
}

func fetchFeedData(httpClient *resty.Client, feedURL string) (*bytes.Buffer, error) {
	// Set up context.
	ctx, cancelFunc := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFunc()

	// Parse the URL to ensure its valid.
	sourceURL, err := url.Parse(feedURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse URL: %w", err)
	}
	if !sourceURL.IsAbs() {
		return nil, fmt.Errorf("not an absolute URL: %s", sourceURL.String())
	}

	resp, err := httpClient.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(sourceURL.String())
	switch {
	case err != nil:
		return nil, fmt.Errorf("fetch feed: %w", err)
	case resp.IsError():
		return nil, fmt.Errorf("fetch feed response: %s", resp.Status())
	}
	defer resp.RawBody().Close()

	// Read response data into buffer.
	var feedBuf bytes.Buffer
	if resp.Header().Get("Content-Encoding") == "gzip" {
		// For gzipped response, uncompress first.
		reader, err := gzip.NewReader(resp.RawBody())
		if err != nil {
			return nil, fmt.Errorf("read gzip response: %w", err)
		}
		defer reader.Close()
		const maxBodySize = 10 * 1024 * 1024 // 10 MB limit
		limitReader := io.LimitReader(reader, maxBodySize)
		if _, err := io.Copy(&feedBuf, limitReader); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
	} else {
		// Read response directly.
		if _, err := io.Copy(&feedBuf, resp.RawBody()); err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
	}

	return &feedBuf, nil
}
