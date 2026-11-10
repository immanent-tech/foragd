// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package indices

import (
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

// Mappings is an index mapping definition.
type Mappings map[string]opensearchapi.CommonMappingProperty

// MappingOption is a functional option applied to Mappings.
type MappingOption func(Mappings)

// WithMapping option adds a new mapping of type O.
func WithMapping[O any](name string, options ...func(*O)) MappingOption {
	return func(m Mappings) {
		settings := new(O)
		for option := range slices.Values(options) {
			option(settings)
		}
		// TODO: add all used mapping properties.
		switch value := any(settings).(type) {
		case *opensearchapi.CommonMappingTextProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromTextProperty(*value)
		case *opensearchapi.CommonMappingKeywordProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromKeywordProperty(*value)
		case *opensearchapi.CommonMappingSearchAsYouTypeProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromSearchAsYouTypeProperty(*value)
		case *opensearchapi.CommonMappingDateNanosProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromDateNanosProperty(*value)
		case *opensearchapi.CommonMappingUnsignedLongNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromUnsignedLongNumberProperty(*value)
		case *opensearchapi.CommonMappingFlatObjectProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromFlatObjectProperty(*value)
		}
	}
}

// TextMappingOption is a functional option applied to a text mapping.
type TextMappingOption func(*opensearchapi.CommonMappingTextProperty)

// WithTextMappingProperties option sets additional mapping properties (i.e., multi-fields) on a text mapping.
func WithTextMappingProperties(props ...MappingOption) TextMappingOption {
	return func(p *opensearchapi.CommonMappingTextProperty) {
		fields := make(map[string]opensearchapi.CommonMappingProperty)
		for props := range slices.Values(props) {
			props(fields)
		}
		p.Fields = fields
	}
}

// WithTextMappingAnalyzer option sets the analyzer to use for the text mapping. Note that the analyzer must be present
// in the analysis settings of the index, otherwise this will cause an error at index time.
func WithTextMappingAnalyzer(name string) TextMappingOption {
	return func(p *opensearchapi.CommonMappingTextProperty) {
		p.Analyzer = &name
	}
}

// WithTextMappingCopyTo option will copy the value indexed in the text mapping to the named field.
func WithTextMappingCopyTo(to string) TextMappingOption {
	return func(p *opensearchapi.CommonMappingTextProperty) {
		p.CopyTo = &to
	}
}

type CustomAnalyzer struct {
	Name       string
	Definition opensearchapi.CommonAnalysisAnalyzer
}
