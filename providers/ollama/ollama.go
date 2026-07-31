// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package ollama

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/immanent-tech/go-base/client"

	"github.com/immanent-tech/foragd/providers/elastic/vector"
)

type ollamaEmbedRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	KeepAlive string   `json:"keep_alive,omitempty"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Error      string      `json:"error,omitempty"`
}

// EmbedBatch embeds up to BatchSize texts in one request against a local Ollama server. Ollama returns L2-normalized
// vectors already, so no further normalization is needed for cosine similarity.
func EmbedBatch(texts ...string) ([][]float32, error) {
	if err := LoadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Create request body.
	reqBody := ollamaEmbedRequest{
		Model:     cfg.Model,
		Input:     texts,
		KeepAlive: cfg.KeepAlive.String(),
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	client, err := client.Load()
	if err != nil {
		return nil, fmt.Errorf("load client: %w", err)
	}

	var res ollamaEmbedResponse
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		SetResult(&res).
		SetError(&res).
		// SetDebug(true).
		Post(cfg.URL)
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

// EmbedChunks embeds all chunks, batching requests according to c.BatchSize.
func EmbedChunks(chunks ...vector.Chunk) ([][]float32, error) {
	if err := LoadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	all := make([][]float32, len(chunks))
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	for start := 0; start < len(chunks); start += batchSize {
		end := min(start+batchSize, len(chunks))

		texts := make([]string, end-start)
		for i := start; i < end; i++ {
			texts[i-start] = chunks[i].Text
		}

		vectors, err := EmbedBatch(texts...)
		if err != nil {
			return nil, fmt.Errorf("embedding batch [%d:%d]: %w", start, end, err)
		}
		copy(all[start:end], vectors)
	}

	return all, nil
}

func cosineSimilarity(a, b []float32) float32 {
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(mathSqrt(normA)) * float32(mathSqrt(normB)))
}

func mathSqrt(f float32) float64 {
	return math.Sqrt(float64(f))
}
