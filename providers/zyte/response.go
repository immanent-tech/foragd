// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package zyte

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"

	"codeberg.org/readeck/go-readability/v2"

	"github.com/immanent-tech/foragd/validation"
)

// GetHTMLResponse retrieves the response body from a html request (created by the httpResponseBody Request option).
func (r *Response) GetHTMLResponse() ([]byte, error) {
	if r.HttpResponseBody != nil {
		body, err := base64.StdEncoding.DecodeString(*r.HttpResponseBody)
		if err != nil {
			return nil, fmt.Errorf("decode body: %w", err)
		}
		return validation.SanitizeBytes(body), nil
	}
	return nil, ErrNotFound
}

// GetBrowserResponse retrieves the response body from a browser request (created by the browserHtml Request option).
func (r *Response) GetBrowserResponse() ([]byte, error) {
	if r.BrowserHtml != nil {
		return []byte(validation.SanitizeString(*r.BrowserHtml)), nil
	}
	return nil, ErrNotFound
}

func (r *Response) GetURL() (*url.URL, error) {
	if pageURL, err := url.Parse(r.URL); err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	} else {
		return pageURL, nil
	}
}

// ExtractArticle extracts an Article from the Response using the readability package. It extracts the content from
// either the responseHtml or browserHtml field of the response.
func (r *Response) ExtractArticle() (*Article, error) {
	var body []byte
	htmlBody, htmlErr := r.GetHTMLResponse()
	browserBody, browserErr := r.GetBrowserResponse()
	switch {
	case browserErr == nil:
		// Use browserBody.
		body = browserBody
	case htmlErr == nil:
		// Use htmlResponseBody.
		body = htmlBody
	default:
		return nil, fmt.Errorf("get body: %w", errors.Join(htmlErr, browserErr))
	}

	pageURL, err := r.GetURL()
	if err != nil {
		return nil, fmt.Errorf("get url: %w", err)
	}
	extracted, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		return nil, fmt.Errorf("extract article: %w", err)
	}

	// Copy extracted content to buffers.
	textBuf, ok := bufPool.Get().(*bytes.Buffer)
	if ok {
		textBuf.Reset()
		defer bufPool.Put(textBuf)
		if err := extracted.RenderText(textBuf); err != nil {
			return nil, fmt.Errorf("render text: %w", err)
		}
	} else {
		return nil, fmt.Errorf("get text buffer: %w", err)
	}
	htmlBuf, ok := bufPool.Get().(*bytes.Buffer)
	if ok {
		htmlBuf.Reset()
		defer bufPool.Put(htmlBuf)
		if err := extracted.RenderHTML(htmlBuf); err != nil {
			return nil, fmt.Errorf("render html: %w", err)
		}
	} else {
		return nil, fmt.Errorf("get html buffer: %w", err)
	}

	article := &Article{
		ArticleBody:     new(textBuf.String()),
		ArticleBodyHtml: new(htmlBuf.String()),
		URL:             pageURL.String(),
		Description:     new(extracted.Excerpt()),
		Headline:        new(extracted.Title()),
	}

	author := Author{
		Name: extracted.Byline(),
	}
	article.Authors = []Author{author}

	if modified, err := extracted.ModifiedTime(); err != nil {
		article.DateModified = new(modified.UTC().Format(time.RFC3339))
	}

	if published, err := extracted.PublishedTime(); err != nil {
		article.DatePublished = new(published.UTC().Format(time.RFC3339))
	}

	return article, nil
}
