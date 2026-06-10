// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/frontmatter"
)

var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var LoadMarkdownWriter = sync.OnceValue(func() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			&frontmatter.Extender{},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
})

// ToHTML treats the given string data input as markdown formatted plain-text and returns an appropriate HTML
// representation.
func ToHTML(input []byte) ([]byte, error) {
	converter := LoadMarkdownWriter()
	buf, ok := bufPool.Get().(*bytes.Buffer)
	if !ok {
		return input, errors.New("unable to retrieve buffer")
	}
	defer func() {
		buf.Reset()
		defer bufPool.Put(buf)
	}()
	if err := converter.Convert(input, buf); err != nil {
		return nil, fmt.Errorf("format as markdown: %w", err)
	}
	return buf.Bytes(), nil
}
