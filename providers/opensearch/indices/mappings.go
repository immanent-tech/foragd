// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package indices

import (
	"context"
	"slices"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch"
)

// Mappings is an index mapping definition.
type Mappings map[string]opensearchapi.CommonMappingProperty

// NewMappings creates a set of mappings. The [Mappings] object created can be used in template creation or for mapping
// update requests.
func NewMappings(options ...MappingOption) Mappings {
	mappings := make(Mappings)
	for option := range slices.Values(options) {
		option(mappings)
	}
	return mappings
}

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
		case *opensearchapi.CommonMappingByteNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromByteNumberProperty(*value)
		case *opensearchapi.CommonMappingShortNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromShortNumberProperty(*value)
		case *opensearchapi.CommonMappingIntegerNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromIntegerNumberProperty(*value)
		case *opensearchapi.CommonMappingUnsignedLongNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromUnsignedLongNumberProperty(*value)
		case *opensearchapi.CommonMappingLongNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromLongNumberProperty(*value)
		case *opensearchapi.CommonMappingFloatNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromFloatNumberProperty(*value)
		case *opensearchapi.CommonMappingDoubleNumberProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromDoubleNumberProperty(*value)
		case *opensearchapi.CommonMappingBooleanProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromBooleanProperty(*value)
		case *opensearchapi.CommonMappingBinaryProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromBinaryProperty(*value)
		case *opensearchapi.CommonMappingFlatObjectProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromFlatObjectProperty(*value)
		case *opensearchapi.CommonMappingObjectProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromObjectProperty(*value)
		case *opensearchapi.CommonMappingGeoPointProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromGeoPointProperty(*value)
		case *opensearchapi.CommonMappingGeoShapeProperty:
			m[name] = opensearchapi.NewCommonMappingPropertyFromGeoShapeProperty(*value)
		}
	}
}

// TextMappingOption is a functional option applied to a text mapping.
type TextMappingOption func(*opensearchapi.CommonMappingTextProperty)

// WithTextMappingProperties option sets additional mapping properties (i.e., multi-fields) on a text mapping.
func WithTextMappingProperties(props ...MappingOption) TextMappingOption {
	return func(p *opensearchapi.CommonMappingTextProperty) {
		p.Fields = make(map[string]opensearchapi.CommonMappingProperty)
		for props := range slices.Values(props) {
			props(p.Fields)
		}
	}
}

// WithTextMappingAnalyzer option sets the analyzer to use for the text mapping. Note that the analyzer must be present
// in the analysis settings of the index, otherwise this will cause an error at index time.
func WithTextMappingAnalyzer(analyzer CustomAnalyzer) TextMappingOption {
	return func(p *opensearchapi.CommonMappingTextProperty) {
		p.Analyzer = &analyzer.Name
	}
}

// WithTextMappingCopyTo option will copy the value indexed in the text mapping to the named field.
func WithTextMappingCopyTo(to string) TextMappingOption {
	return func(p *opensearchapi.CommonMappingTextProperty) {
		p.CopyTo = &to
	}
}

type ObjectMappingOption func(*opensearchapi.CommonMappingObjectProperty)

func WithObjectMappingProperties(props ...MappingOption) ObjectMappingOption {
	return func(p *opensearchapi.CommonMappingObjectProperty) {
		p.Properties = make(map[string]opensearchapi.CommonMappingProperty)
		for props := range slices.Values(props) {
			props(p.Properties)
		}
	}
}

func WithObjectMappingDynamic(value opensearchapi.CommonMappingDynamic) ObjectMappingOption {
	return func(p *opensearchapi.CommonMappingObjectProperty) {
		p.Dynamic = &value
	}
}

// KeywordMappingOption is a functional option to apply to a keyword mapping.
type KeywordMappingOption func(*opensearchapi.CommonMappingKeywordProperty)

// WithKeywordMappingNormalizer option sets a normalizer to use on the keyword mapping.
func WithKeywordMappingNormalizer(normalizer CustomNormalizer) KeywordMappingOption {
	return func(p *opensearchapi.CommonMappingKeywordProperty) {
		p.Normalizer = &normalizer.Name
	}
}

type CustomAnalyzer struct {
	Name       string
	Definition opensearchapi.CommonAnalysisAnalyzer
}

type CustomTokenizer struct {
	Name       string
	Definition opensearchapi.CommonAnalysisTokenizer
}

type CustomNormalizer struct {
	Name       string
	Definition opensearchapi.CommonAnalysisNormalizer
}

// UpdateIndexMappings will update existing mappings in the given index.
func UpdateIndexMappings(
	ctx context.Context,
	client opensearchapi.IndicesClient,
	index string,
	mappings opensearchapi.IndicesPutMappingBody,
) error {
	req := opensearchapi.IndicesPutMappingReq{
		Indices: []string{index},
		Body:    &mappings,
		Header:  opensearch.NewRequestHeaders(ctx),
	}
	// Update the mappings.
	resp, err := client.PutMapping(ctx, &req)
	if err != nil {
		return opensearch.HandleError("updating mappings", resp, err)
	}
	return nil
}
