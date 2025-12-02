// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package ingest

import (
	"github.com/elastic/go-elasticsearch/v9/typedapi/ingest/putpipeline"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

// Option is a functional option for a reindex operation.
type Option func(*putpipeline.Request)

func WithProcessor(processer types.ProcessorContainer) Option {
	return func(r *putpipeline.Request) {
		r.Processors = append(r.Processors, processer)
	}
}

// NewIngestPipeline creates a new ingest pipeline from the given options.
func NewIngestPipeline(options ...Option) *putpipeline.Request {
	pipeline := &putpipeline.Request{}

	for _, option := range options {
		option(pipeline)
	}

	return pipeline
}
