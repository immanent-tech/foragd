// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v9/typedapi/ingest/putpipeline"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

// RemoveDescription adds the given description to the remove processor.
func RemoveDescription(description string) Option[*types.RemoveProcessor] {
	return func(processor *types.RemoveProcessor) *types.RemoveProcessor {
		processor.Description = &description
		return processor
	}
}

// RemoveFields will mark the given fields to be removed with the remove processor.
func RemoveFields(fields ...string) Option[*types.RemoveProcessor] {
	return func(processor *types.RemoveProcessor) *types.RemoveProcessor {
		processor.Field = fields
		return processor
	}
}

// RemoveIgnoreMissing sets whether the remove processor should ignore missing fields.
func RemoveIgnoreMissing(value bool) Option[*types.RemoveProcessor] {
	return func(processor *types.RemoveProcessor) *types.RemoveProcessor {
		processor.IgnoreMissing = &value
		return processor
	}
}

// WithRemoveProcessor adds the given remove processor configuration to the pipeline.
func WithRemoveProcessor(options ...Option[*types.RemoveProcessor]) Option[*putpipeline.Request] {
	return func(pipeline *putpipeline.Request) *putpipeline.Request {
		remove := &types.RemoveProcessor{}

		for _, option := range options {
			remove = option(remove)
		}

		pipeline.Processors = append(pipeline.Processors, types.ProcessorContainer{Remove: remove})

		return pipeline
	}
}

// NewIngestPipeline creates a new ingest pipeline from the given options.
func NewIngestPipeline(options ...Option[*putpipeline.Request]) *putpipeline.Request {
	pipeline := &putpipeline.Request{}

	for _, option := range options {
		pipeline = option(pipeline)
	}

	return pipeline
}
