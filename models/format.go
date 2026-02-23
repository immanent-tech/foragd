// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package models

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Common HTML tags.
var htmlTags = []string{
	// Document structure
	"html", "head", "body", "doctype",
	// Metadata
	"title", "meta", "link", "style", "script",
	// Sectioning
	"header", "footer", "main", "nav", "section", "article", "aside",
	// Block elements
	"div", "p", "h1", "h2", "h3", "h4", "h5", "h6",
	"ul", "ol", "li", "dl", "dt", "dd",
	"table", "thead", "tbody", "tfoot", "tr", "th", "td",
	"form", "fieldset", "legend", "label",
	"blockquote", "pre", "figure", "figcaption", "hr",
	// Inline elements
	"a", "span", "strong", "em", "b", "i", "u", "s",
	"img", "input", "button", "select", "textarea", "option",
	"code", "kbd", "samp", "var", "abbr", "cite",
	"br", "small", "sub", "sup",
	// Media / embedded
	"video", "audio", "canvas", "iframe", "svg",
}

var (
	// Matches a DOCTYPE declaration.
	doctypeRe = regexp.MustCompile(`(?i)<!DOCTYPE\s+html`)
	// Matches HTML comments.
	commentRe = regexp.MustCompile(`<!--[\s\S]*?-->`)
	// Matches any opening or closing HTML tag from our known list,
	// with optional attributes. e.g. <div>, <a href="...">, </p>.
	tagPattern = buildTagPattern()
)

var loadMarkdownWriter = sync.OnceValue(func() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
})

func FormatAsMarkdown(input []byte) ([]byte, error) {
	converter := loadMarkdownWriter()
	var buf bytes.Buffer
	if err := converter.Convert(input, &buf); err != nil {
		return nil, fmt.Errorf("format as markdown: %w", err)
	}
	return buf.Bytes(), nil
}

// IsHTML returns a boolean indicating whether the given string contains HTML. It can detect both a full HTML document
// or partial HTML content.
func IsHTML(s string) bool {
	score := 0

	trimmed := strings.TrimSpace(s)
	if len(trimmed) == 0 {
		return false
	}

	lower := strings.ToLower(trimmed)

	// Signal 1: DOCTYPE declaration — very strong indicator
	if doctypeRe.MatchString(trimmed) {
		score += 40
	}

	// Signal 2: <html> tag present
	if strings.Contains(lower, "<html") {
		score += 30
	}

	// Signal 3: <head> + <body> structure
	hasHead := strings.Contains(lower, "<head")
	hasBody := strings.Contains(lower, "<body")
	if hasHead && hasBody {
		score += 20
	} else if hasHead || hasBody {
		score += 10
	}

	// Signal 4: Count known HTML tag matches
	matches := tagPattern.FindAllString(trimmed, -1)
	if tagCount := len(matches); tagCount >= 3 {
		switch {
		case tagCount >= 10:
			score += 30
		case tagCount >= 5:
			score += 20
		default:
			score += 10
		}
	} else if tagCount > 0 {
		score += 5
	}

	// Signal 5: HTML comment syntax
	if commentRe.MatchString(trimmed) {
		score += 10
	}

	// Signal 6: Common HTML attribute patterns (href, src, class, id, style)
	attrRe := regexp.MustCompile(`(?i)\s(?:href|src|class|id|style|alt|type|name|value|placeholder)\s*=\s*["']`)
	if attrRe.MatchString(trimmed) {
		score += 10
	}

	// Signal 7: Self-closing tags like <br/>, <img/>, <input/>
	selfClosingRe := regexp.MustCompile(`(?i)<(?:br|hr|img|input|meta|link)\b[^>]*?/?>`)
	if selfClosingRe.MatchString(trimmed) {
		score += 5
	}

	// Signal 8: Starts with a tag (strong partial HTML indicator)
	if bytes.HasPrefix([]byte(trimmed), []byte("<")) && tagPattern.MatchString(trimmed[:min(50, len(trimmed))]) {
		score += 10
	}

	// Normalise score to a 0–1 confidence value (cap at 100 before dividing)
	if score > 100 {
		score = 100
	}
	confidence := float64(score) / 100.0
	return confidence >= 0.10 // low threshold — we want to catch partials
}

func buildTagPattern() *regexp.Regexp {
	joined := strings.Join(htmlTags, "|")
	// Match opening tags (with optional attrs) or closing tags
	pattern := `(?i)<(/?)(?:` + joined + `)(?:\s[^>]*)?>`
	return regexp.MustCompile(pattern)
}
