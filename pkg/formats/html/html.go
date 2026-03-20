// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package html

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/immanent-tech/go-syndication/client"
	"golang.org/x/net/html"
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
	if regexp.MustCompile(`(?i)\s(?:href|src|class|id|style|alt|type|name|value|placeholder)\s*=\s*["']`).
		MatchString(trimmed) {
		score += 10
	}

	// Signal 7: Self-closing tags like <br/>, <img/>, <input/>
	if regexp.MustCompile(`(?i)<(?:br|hr|img|input|meta|link)\b[^>]*?/?>`).MatchString(trimmed) {
		score += 5
	}

	// Signal 8: Starts with a tag (strong partial HTML indicator)
	if strings.HasPrefix(trimmed, "<") && tagPattern.MatchString(trimmed[:min(50, len(trimmed))]) {
		score += 10
	}

	// Normalise score to a 0–1 confidence value (cap at 100 before dividing)
	if score > 100 {
		score = 100
	}
	confidence := float64(score) / 100.0
	return confidence >= 0.10 // low threshold — we want to catch partials
}

// IsHTMLElement returns a boolean indicating whether the given string is the given HTML element.
func IsHTMLElement(str, tag string) bool {
	pattern1 := regexp.MustCompile(`(?i)<(/?)(?:` + tag + `)(?:\s[^>]*)?>`)
	pattern2 := regexp.MustCompile(`(?i)<(?:` + tag + `)\b[^>]*?/?>`)
	trimmed := strings.TrimSpace(str)
	switch {
	case len(trimmed) == 0:
		return false
	case strings.HasPrefix(trimmed, "<") && pattern1.MatchString(trimmed[:min(50, len(trimmed))]):
		return true
	case pattern2.MatchString(trimmed):
		return true
	default:
		return false
	}
}

// FindHTMLNode does a depth-first search for the first node matching the tag.
func FindHTMLNode(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := FindHTMLNode(c, tag); result != nil {
			return result
		}
	}
	return nil
}

// FindAllHTMLNodes returns all nodes matching the tag within n.
func FindAllHTMLNodes(n *html.Node, tag string) []*html.Node {
	var results []*html.Node
	if n.Type == html.ElementNode && n.Data == tag {
		results = append(results, n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		results = append(results, FindAllHTMLNodes(c, tag)...)
	}
	return results
}

func buildTagPattern() *regexp.Regexp {
	joined := strings.Join(htmlTags, "|")
	// Match opening tags (with optional attrs) or closing tags
	pattern := `(?i)<(/?)(?:` + joined + `)(?:\s[^>]*)?>`
	return regexp.MustCompile(pattern)
}

// Favicon is a favicon link found in <head>.
type Favicon struct {
	href string
	rel  string // e.g. "icon", "apple-touch-icon", "shortcut icon"
	typ  string // e.g. "image/png"
	size string // e.g. "32x32"
}

// findFaviconCandidates fetches the page and parses <link> tags in <head> that look
// like favicon declarations, plus synthesises the conventional /favicon.ico path.
func findFaviconCandidates(page []byte) []Favicon {
	limited := client.NewHeadReader(bytes.NewReader(page), 256*1024)

	tokenizer := html.NewTokenizer(limited)

	var candidates []Favicon
	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tok := tokenizer.Token()
		if tok.Data == "body" {
			break
		}
		if tok.Data != "link" {
			continue
		}

		var rel, href, typ, size string
		for _, a := range tok.Attr {
			switch strings.ToLower(a.Key) {
			case "rel":
				rel = strings.ToLower(a.Val)
			case "href":
				href = a.Val
			case "type":
				typ = a.Val
			case "sizes":
				size = a.Val
			}
		}

		if href == "" {
			continue
		}
		// Accept any rel that contains "icon"
		if !strings.Contains(rel, "icon") {
			continue
		}
		candidates = append(candidates, Favicon{href: href, rel: rel, typ: typ, size: size})
	}

	// Always append the conventional fallback last.
	candidates = append(candidates, Favicon{href: "/favicon.ico", rel: "conventional"})
	return candidates
}

// resolve turns a possibly relative href into an absolute URL based on the page origin.
func resolve(pageURL, href string) (string, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", fmt.Errorf("parse url %s: %w", pageURL, err)
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", fmt.Errorf("parse url %s: %w", href, err)
	}
	return base.ResolveReference(ref).String(), nil
}

// FindFavicon tries each candidate in order and returns the first one that
// responds with a 2xx status and a non-empty body.
func FindFavicon(
	page []byte,
	pageURL string,
) ([]byte, string, Favicon, error) {
	candidates := findFaviconCandidates(page)
	if len(candidates) == 0 {
		return nil, "", Favicon{}, errors.New("no favicon candidates found")
	}

	for _, cand := range candidates {
		abs, err := resolve(pageURL, cand.href)
		if err != nil {
			continue
		}
		resp, err := client.LoadHTTPClient().R().Get(abs)
		if err != nil {
			continue
		}
		if resp.StatusCode() < 200 || resp.StatusCode() >= 300 || len(resp.Body()) == 0 {
			continue
		}
		return resp.Body(), abs, cand, nil
	}
	return nil, "", Favicon{}, errors.New("no reachable favicon found")
}

// FindMainImage tries to find a "main" image for the page, using the readability parser.
func FindMainImage(page []byte, rawURL string) (string, error) {
	pageURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}

	node, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}
	// Parse using readability to find main content details.
	rdData, err := readability.FromDocument(node, pageURL)
	if err != nil {
		return "", fmt.Errorf("find image: %w", err)
	}
	if rdData.ImageURL() == "" {
		return "", errors.New("no main image found")
	}
	return rdData.ImageURL(), nil
}

// SanitizeHTMLString will parse and re-render the given string containing HTML. In doing so, the HTML is hopefully
// sanitized and reformatted to be well-formed HTML.
func SanitizeHTMLString(rawStr string) (string, error) {
	rawHTML, err := html.Parse(strings.NewReader(rawStr))
	if err != nil {
		return "", fmt.Errorf("unable to parse data as HTML: %w", err)
	}

	buf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return "", errors.New("unable to retrieve buffer")
	}
	defer func() {
		buf.Reset()
		defer bufPool.Put(buf)
	}()

	if err := html.Render(buf, rawHTML); err != nil {
		return "", fmt.Errorf("cannot write sanitized HTML: %w", err)
	}

	return buf.String(), nil
}

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}
