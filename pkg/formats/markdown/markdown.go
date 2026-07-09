// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

//go:generate go tool oapi-codegen -config markdown-cfg.yaml markdown.yaml
package markdown

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/frontmatter"

	"github.com/immanent-tech/foragd/config"
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

// ToHTML treats the given string data input as Markdown formatted plain-text and returns an appropriate HTML
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

func (fm *FrontMatter) GetCreatedDate() time.Time {
	created, _ := time.Parse(time.DateOnly, fm.CreatedAt)
	return created
}

func (fm *FrontMatter) GetUpdatedDate() time.Time {
	if fm.UpdatedAt != nil {
		updated, _ := time.Parse(time.DateOnly, *fm.UpdatedAt)
		return updated
	}
	return time.Time{}
}

// ReadDir reads a list of Markdown files from the given path in an embed.FS.
func ReadDir(dir embed.FS, path string) ([]*File, error) {
	// Get the list of files at the given path.
	fileList, err := dir.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	// Extract each file's Markdown content.
	files := make([]*File, 0, len(fileList))
	for file := range slices.Values(fileList) {
		policy, err := ReadFile(dir, path, file)
		if err != nil {
			slog.Warn("Could not read file.",
				slog.Any("error", err))
			continue
		}
		files = append(files, policy)
	}

	return files, nil
}

// ReadFile reads a file at the given path in an embed.FS.
func ReadFile(dir embed.FS, path string, details fs.DirEntry) (*File, error) {
	contents, err := dir.ReadFile(filepath.Join(path, details.Name()))
	if err != nil {
		return nil, fmt.Errorf("read file contents: %w", err)
	}

	mdw := LoadMarkdownWriter()
	var buf bytes.Buffer

	parserCtx := parser.NewContext()
	if err := mdw.Convert(contents, &buf, parser.WithContext(parserCtx)); err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}

	d := frontmatter.Get(parserCtx)
	var fm FrontMatter
	if err := d.Decode(&fm); err != nil {
		return nil, fmt.Errorf("decode frontmatter: %w", err)
	}

	jsonld, err := generateJSONLD(&fm)
	if err != nil {
		return nil, fmt.Errorf("generate json-ld data: %w", err)
	}

	return &File{
		Frontmatter: fm,
		JsonLD:      &jsonld,
		Content:     buf.Bytes(),
	}, nil
}

func generateJSONLD(frontmatter *FrontMatter) (json.RawMessage, error) {
	var img string
	if frontmatter.Image != nil {
		img = config.GetBaseURL() + *frontmatter.Image
	}
	data := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "Article",
		"headline":      frontmatter.Title,
		"description":   frontmatter.Description,
		"image":         img,
		"datePublished": frontmatter.GetCreatedDate(),
		"dateModified":  frontmatter.GetUpdatedDate(),
		"author": map[string]any{
			"@type": "Person",
			"name":  frontmatter.Author,
		},
		"publisher": map[string]any{
			"@type": "Organization",
			"name":  "Foragd",
			"url":   config.GetBaseURL(),
		},
		"mainEntityOfPage": map[string]any{
			"@type": "WebPage",
			"@id":   config.GetBaseURL() + "/blog/" + frontmatter.Slug,
		},
	}

	jsonLD, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}
	return jsonLD, nil
}
