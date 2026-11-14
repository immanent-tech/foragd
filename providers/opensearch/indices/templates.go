// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package indices

import (
	"slices"
	"sync"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
)

// ComponentTemplate is an object that contains information about an component template.
type ComponentTemplate struct {
	*opensearchapi.ClusterPutComponentTemplateBody
}

// NewComponentTemplateRequest creates a new component template request with the given name and options.
func NewComponentTemplateRequest(
	name string,
	options ...ComponentTemplateOption,
) opensearchapi.ClusterPutComponentTemplateReq {
	template := &ComponentTemplate{ClusterPutComponentTemplateBody: &opensearchapi.ClusterPutComponentTemplateBody{}}
	for option := range slices.Values(options) {
		option(template)
	}
	return opensearchapi.ClusterPutComponentTemplateReq{
		Name: name,
		Body: template.ClusterPutComponentTemplateBody,
	}
}

// ComponentTemplateOption is a functional option that applies to a component template.
type ComponentTemplateOption func(*ComponentTemplate)

// WithAllowAutoCreate option specifies whether indexes are auto-created with this template.
func WithAllowAutoCreate(value bool) ComponentTemplateOption {
	return func(t *ComponentTemplate) {
		t.AllowAutoCreate = &value
	}
}

// WithIndexSettings option applies the given index settings to the component template.
func WithIndexSettings(options ...IndexSettingsOption) ComponentTemplateOption {
	return func(t *ComponentTemplate) {
		settings := &IndexSettings{IndicesIndexState: &opensearchapi.IndicesIndexState{
			Mappings: &opensearchapi.CommonMappingType{},
			Settings: &opensearchapi.IndicesIndexSettings{},
		}}
		for option := range slices.Values(options) {
			option(settings)
		}
		t.Template = *settings.IndicesIndexState
	}
}

// IndexSettings defines settings for an index.
type IndexSettings struct {
	*opensearchapi.IndicesIndexState

	mu sync.Mutex
}

func (s *IndexSettings) enableAnalysis() {
	if s.Settings.Analysis == nil {
		s.Settings.Analysis = &opensearchapi.IndicesIndexSettingsAnalysis{}
	}
}

// IndexSettingsOption is a functional option applied to IndexSettings.
type IndexSettingsOption func(*IndexSettings)

// WithReplicas option sets the number of replica shards for the index.
func WithReplicas(count string) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.Settings.NumberOfReplicas = &count
	}
}

// WithKNN option sets the KNN index setting to "true".
func WithKNN() IndexSettingsOption {
	return func(s *IndexSettings) {
		s.Settings.KNN = new("true")
	}
}

// WithDefaultPipeline option sets a default ingest pipeline for the index.
func WithDefaultPipeline(pipeline string) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.Settings.DefaultPipeline = &pipeline
	}
}

// WithAlias option assigns an alias with the given name and settings to the settings.
func WithAlias(name string, settings *opensearchapi.IndicesAlias) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.Aliases == nil {
			s.Aliases = make(map[string]opensearchapi.IndicesAlias)
		}
		if settings != nil {
			s.Aliases[name] = *settings
		} else {
			s.Aliases[name] = opensearchapi.IndicesAlias{}
		}
	}
}

// WithAnalyzer option assigns the given analyzer definition to the settings.
func WithAnalyzer(analyzer CustomAnalyzer) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.enableAnalysis()
		if s.Settings.Analysis.Analyzer == nil {
			s.Settings.Analysis.Analyzer = make(map[string]opensearchapi.CommonAnalysisAnalyzer)
		}
		s.Settings.Analysis.Analyzer[analyzer.Name] = analyzer.Definition
	}
}

// WithTokenizer option assigns the given tokenizer definition to the settings.
func WithTokenizer(tokenizer CustomTokenizer) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.enableAnalysis()
		if s.Settings.Analysis.Tokenizer == nil {
			s.Settings.Analysis.Tokenizer = make(map[string]opensearchapi.CommonAnalysisTokenizer)
		}
		s.Settings.Analysis.Tokenizer[tokenizer.Name] = tokenizer.Definition
	}
}

// WithNormalizer option assigns the given normalizer definition to the settings.
func WithNormalizer(normalizer CustomNormalizer) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.enableAnalysis()
		if s.Settings.Analysis.Normalizer == nil {
			s.Settings.Analysis.Normalizer = make(map[string]opensearchapi.CommonAnalysisNormalizer)
		}
		s.Settings.Analysis.Normalizer[normalizer.Name] = normalizer.Definition
	}
}

// WithMappingProperties option assigns the given mappings definition to the settings.
func WithMappingProperties(props ...MappingOption) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.mu.Lock()
		defer s.mu.Unlock()

		mappings := make(map[string]opensearchapi.CommonMappingProperty)
		for prop := range slices.Values(props) {
			prop(mappings)
		}
		s.Mappings.Properties = mappings
	}
}

// WithMappings option assigns the given mappings to the settings.
func WithMappings(mappings Mappings) IndexSettingsOption {
	return func(s *IndexSettings) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.Mappings.Properties = mappings
	}
}

// IndexTemplate is an object that contains information about an index template.
type IndexTemplate struct {
	*opensearchapi.IndicesPutIndexTemplateBody
}

// NewIndexTemplateRequest creates a new index template request with the given name and options.
func NewIndexTemplateRequest(name string, options ...IndexTemplateOption) opensearchapi.IndicesPutIndexTemplateReq {
	template := &IndexTemplate{IndicesPutIndexTemplateBody: &opensearchapi.IndicesPutIndexTemplateBody{}}
	for option := range slices.Values(options) {
		option(template)
	}
	return opensearchapi.IndicesPutIndexTemplateReq{
		Name: name,
		Body: template.IndicesPutIndexTemplateBody,
	}
}

// IndexTemplateOption is a functional option that applies to an index template.
type IndexTemplateOption func(*IndexTemplate)

// WithComponentTemplates option assigns the given component templates to the index template.
func WithComponentTemplates(templates ...string) IndexTemplateOption {
	return func(t *IndexTemplate) {
		t.ComposedOf = append(t.ComposedOf, templates...)
	}
}

// WithIndexPatterns option assigns the given patterns to the index template.
func WithIndexPatterns(patterns ...string) IndexTemplateOption {
	return func(t *IndexTemplate) {
		t.IndexPatterns = append(t.IndexPatterns, patterns...)
	}
}

// WithIndexTemplatePriority option assigns a priority value to the inex template, which will define the order in which
// it is applied when multiple templates match.
func WithIndexTemplatePriority(priority int) IndexTemplateOption {
	return func(t *IndexTemplate) {
		t.Priority = &priority
	}
}
