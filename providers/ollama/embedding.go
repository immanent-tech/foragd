/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/immanent-tech/foragd/providers/elastic/vector"
)

type ollamaEmbedRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	KeepAlive string   `json:"keep_alive,omitempty"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
}

// embedBatch embeds up to BatchSize texts in one request against a local Ollama server. Ollama returns L2-normalized
// vectors already, so no further normalization is needed for cosine similarity.
func (c *Client) embedBatch(ctx context.Context, texts ...string) ([][]float64, error) {
	// Set timeout
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Create request body.
	reqBody := ollamaEmbedRequest{
		Model:     c.Model,
		Input:     texts,
		KeepAlive: c.KeepAlive.String(),
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var res ollamaEmbedResponse
	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		SetResult(&res).
		SetError(&res).
		// SetDebug(true).
		Post(c.URL)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("ollama request: %s: %s", resp.Status(), res.Error)
	}

	if len(res.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(res.Embeddings))
	}

	return res.Embeddings, nil
}

// embedChunks embeds all chunks, batching requests according to c.BatchSize.
func (c *Client) embedChunks(ctx context.Context, chunks ...vector.Chunk) ([][]float64, error) {
	all := make([][]float64, len(chunks))
	batchSize := c.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	for start := 0; start < len(chunks); start += batchSize {
		end := min(start+batchSize, len(chunks))

		texts := make([]string, end-start)
		for i := start; i < end; i++ {
			texts[i-start] = chunks[i].Text
		}

		vectors, err := c.embedBatch(ctx, texts...)
		if err != nil {
			return nil, fmt.Errorf("embedding batch [%d:%d]: %w", start, end, err)
		}
		copy(all[start:end], vectors)
	}

	return all, nil
}
