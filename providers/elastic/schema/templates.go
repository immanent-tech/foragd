// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// WithComponentTemplates assigns the given component templates to the index template.
func WithComponentTemplates(components ...string) Option[*putindextemplate.Request] {
	return func(template *putindextemplate.Request) *putindextemplate.Request {
		template.ComposedOf = components
		return template
	}
}

// WithIndexPatterns assigns the given index patterns to the index template.
func WithIndexPatterns(patterns ...string) Option[*putindextemplate.Request] {
	return func(template *putindextemplate.Request) *putindextemplate.Request {
		template.IndexPatterns = patterns
		return template
	}
}

// WithPriority assigns the given priority to the index template.
func WithPriority(priority int64) Option[*putindextemplate.Request] {
	return func(template *putindextemplate.Request) *putindextemplate.Request {
		template.Priority = &priority
		return template
	}
}

// AsDataStream will associate the index template with a data stream.
func AsDataStream() Option[*putindextemplate.Request] {
	return func(template *putindextemplate.Request) *putindextemplate.Request {
		template.DataStream = types.NewDataStreamVisibility()
		return template
	}
}

func NewIndexTemplateRequest(options ...Option[*putindextemplate.Request]) *putindextemplate.Request {
	template := &putindextemplate.Request{
		Meta_: types.Metadata{

		},
	}

	for _, option := range options {
		template = option(template)
	}

	return template
}

func NewComponentTemplateRequest(template types.IndexState) *putcomponenttemplate.Request {
	return &putcomponenttemplate.Request{
		Template: template,
	}
}
