// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package vector

import (
	"bytes"
	"strings"

	"github.com/immanent-tech/go-base/pkg/htmlx"
)

// Chunk represents a single piece of the original text along with its
// position in the source document.
type Chunk struct {
	Index     int    `json:"index"`
	Text      string `json:"text"`
	ByteStart int    `json:"byte_start"`
	ByteEnd   int    `json:"byte_end"`
}

type EmbeddedChunk struct {
	Chunk

	Embedding []float32 `json:"embedding"`
}

// ChunkBytes splits data into overlapping chunks of at most chunkSize bytes, preferring to break on paragraph, then
// sentence, then whitespace boundaries so words are not sliced in half. overlap bytes from the end of one chunk are
// repeated at the start of the next chunk to preserve context across chunk boundaries (useful for retrieval quality).
func ChunkBytes(data []byte, chunkSize, overlap int) []Chunk {
	if chunkSize <= 0 {
		chunkSize = 2000
	}
	if overlap < 0 || overlap >= chunkSize {
		overlap = chunkSize / 10 // sane default: 10% overlap
	}

	if htmlx.IsHTML(string(data)) {
		// Silently ignore conversion to plain text failing. We just end up with the raw HTML.
		str, _ := htmlx.ToPlainText(data)
		data = []byte(str)
	}

	var chunks []Chunk
	n := len(data)
	start := 0
	idx := 0

	for start < n {
		end := start + chunkSize
		if end >= n {
			end = n
		} else {
			end = bestBreakPoint(data, start, end)
		}

		// Guard against a degenerate break point that doesn't advance.
		if end <= start {
			end = min(start+chunkSize, n)
		}

		text := strings.TrimSpace(string(data[start:end]))
		if text != "" {
			chunks = append(chunks, Chunk{
				Index:     idx,
				Text:      text,
				ByteStart: start,
				ByteEnd:   end,
			})
			idx++
		}

		if end >= n {
			break
		}

		// Move the window forward, backing up by `overlap` bytes so the
		// next chunk shares some trailing context with this one.
		next := end - overlap
		if next <= start {
			next = end // avoid infinite loop if overlap is too aggressive
		}
		start = next
	}

	return chunks
}

// bestBreakPoint looks backward from `end` (within [start, end]) for the nicest place to cut: a blank line, then
// sentence punctuation, then a plain space. Falls back to `end` (a hard cut) if nothing suitable found.
func bestBreakPoint(data []byte, start, end int) int {
	window := data[start:end]

	if i := bytes.LastIndex(window, []byte("\n\n")); i > 0 {
		return start + i + 2
	}
	for _, sep := range [][]byte{[]byte(". "), []byte(".\n"), []byte("! "), []byte("? ")} {
		if i := bytes.LastIndex(window, sep); i > 0 {
			return start + i + len(sep)
		}
	}
	if i := bytes.LastIndexByte(window, '\n'); i > 0 {
		return start + i + 1
	}
	if i := bytes.LastIndexByte(window, ' '); i > 0 {
		return start + i + 1
	}
	return end
}
