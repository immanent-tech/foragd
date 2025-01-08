// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/dynamicmapping"
)

// WithMappingMetadata adds the given version and description strings to the mapping
// metadata.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/mapping-fields.html
func WithMappingMetadata(version, description string) Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Meta_ = types.Metadata{
			"version":     json.RawMessage(strconv.Quote(version)),
			"description": json.RawMessage(strconv.Quote(description)),
		}

		return mapping
	}
}

// WithoutDynamicMapping disables dynamic mapping.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/dynamic-mapping.html
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/explicit-mapping.html
func WithoutDynamicMapping() Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Dynamic = &dynamicmapping.False
		return mapping
	}
}

// WithDataNanosProperty adds a `date_nanos` property to the mapping.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/date_nanos.html
func WithDateNanosProperty(name string) Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Properties[name] = types.NewDateNanosProperty()
		return mapping
	}
}

// WithKeywordProperty adds a `keyword` property to the mapping.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/keyword.html
func WithKeywordProperty(name string) Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Properties[name] = types.NewKeywordProperty()
		return mapping
	}
}

// WithKeywordProperty adds a `text` property to the mapping.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/text.html
func WithTextProperty(name string) Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Properties[name] = types.NewTextProperty()
		return mapping
	}
}

// WithTextAndKeywordProperty adds a property to the mapping that is `text`, and
// has a sub-field `name.raw` that is `keyword`.
func WithTextAndKeywordProperty(name string) Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Properties[name] = asTextAndKeyword()
		return mapping
	}
}

// WithFlattenedProperty adds a `flattened` property to the mapping.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/flattened.html
func WithFlattenedProperty(name string) Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Properties[name] = types.NewFlattenedProperty()
		return mapping
	}
}

// WithObjectProperty adds an `object` property to the mapping.
//
// https://www.elastic.co/guide/en/elasticsearch/reference/current/object.html
func WithObjectProperty(name string, props map[string]types.Property) Option[*types.TypeMapping] {
	return func(mapping *types.TypeMapping) *types.TypeMapping {
		mapping.Properties[name] = types.ObjectProperty{
			Properties: props,
		}

		return mapping
	}
}

// NewPropertyMapping generates a new mapping of document fields from the given options.
func NewPropertyMapping(options ...Option[*types.TypeMapping]) *types.TypeMapping {
	mapping := &types.TypeMapping{}

	for _, option := range options {
		mapping = option(mapping)
	}

	return mapping
}

// asTextAndKeyword is an Elasticsearch field mapping for text fields that
// maps the field as both text and keyword types.
func asTextAndKeyword() types.TextProperty {
	return types.TextProperty{
		Type: "text",
		Fields: map[string]types.Property{
			"raw": types.NewKeywordProperty(),
		},
	}
}
