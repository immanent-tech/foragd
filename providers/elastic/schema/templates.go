// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"context"
	"fmt"
	"slices"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/cluster/putcomponenttemplate"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/dynamicmapping"
)

// Template represents an index or component template with settings, mappings, aliases, etc.
type Template struct {
	types.IndexState
}

// TemplateOption is a functional option to apply to a template.
type TemplateOption Option[*Template]

// NewTemplate creates a new template with the given options.
func NewTemplate(options ...TemplateOption) *Template {
	t := &Template{}
	for _, option := range options {
		option(t)
	}
	return t
}

// WithAlias option assigns an alias with the given name and settings to the template.
func WithAlias(name string, settings *types.Alias) TemplateOption {
	return func(template *Template) {
		if template.Aliases == nil {
			template.Aliases = make(map[string]types.Alias)
		}
		if settings == nil {
			template.Aliases[name] = *types.NewAlias()
		} else {
			template.Aliases[name] = *settings
		}
	}
}

// WithTemplateSettings option applies the given settings options to the template.
func WithTemplateSettings(options ...SettingsOption) TemplateOption {
	return func(t *Template) {
		t.Settings = NewSettings(options...).IndexSettings
	}
}

// Settings represents the settings for a template.
type Settings struct {
	*types.IndexSettings
}

// SettingsOption is a functional option to apply a template setting.
type SettingsOption Option[*Settings]

// NewSettings creates new settings with the given options.
func NewSettings(options ...SettingsOption) *Settings {
	settings := &Settings{
		IndexSettings: types.NewIndexSettings(),
	}
	for _, option := range options {
		option(settings)
	}
	return settings
}

// WithAnalysis option will apply the provided analysis settings.
func WithAnalysis(analysis types.IndexSettingsAnalysis) SettingsOption {
	return func(s *Settings) {
		s.Analysis = &analysis
	}
}

// WithMode option will apply the given index mode to the template.
func WithMode(mode string) SettingsOption {
	return func(s *Settings) {
		s.Mode = &mode
	}
}

// WithLifecycle option will ensure the template applies the given lifecycle policy name to indices.
func WithLifecycle(name string) SettingsOption {
	return func(s *Settings) {
		s.Lifecycle = types.NewIndexSettingsLifecycle()
		s.Lifecycle.Name = &name
	}
}

// WithTemplateMapping option applies the given mapping options to the template.
func WithTemplateMapping(options ...MappingsOption) TemplateOption {
	return func(t *Template) {
		t.Mappings = NewMappings(options...).TypeMapping
	}
}

// Mappings represents the mappings for a template.
type Mappings struct {
	*types.TypeMapping
}

// MappingsOption is a functional option to apply a mappings setting.
type MappingsOption Option[*Mappings]

// NewMappings creates new mappings with the given options.
func NewMappings(options ...MappingsOption) *Mappings {
	mappings := &Mappings{
		TypeMapping: types.NewTypeMapping(),
	}
	for option := range slices.Values(options) {
		option(mappings)
	}
	return mappings
}

// WithProperties option sets field properties in the mapping.
func WithProperties(options ...PropertiesOption) MappingsOption {
	return func(m *Mappings) {
		m.Properties = NewProperties(options...)
	}
}

// WithMetadata option sets metadata for the properties mapping.
func WithMetadata(metadata types.Metadata) MappingsOption {
	return func(m *Mappings) {
		m.Meta_ = metadata
	}
}

// WithDynamicProperties option sets whether dynamic properties are allowed.
func WithDynamicProperties(value bool) MappingsOption {
	return func(m *Mappings) {
		switch value {
		case true:
			m.Dynamic = &dynamicmapping.True
		case false:
			m.Dynamic = &dynamicmapping.False
		}
	}
}

// Properties represents the mapping of field properties.
type Properties map[string]types.Property

// PropertiesOption is a functional option to set the properties for a field.
type PropertiesOption Option[Properties]

// NewProperties creates a set of field mappings with the given options.
func NewProperties(options ...PropertiesOption) Properties {
	p := make(map[string]types.Property)
	for option := range slices.Values(options) {
		option(p)
	}
	return p
}

// WithExistingMappings option will merge the given existing mappings with any new mappings created.
func WithExistingMappings(properties Properties) PropertiesOption {
	return func(mp Properties) {
		for k, v := range properties {
			mp[k] = v
		}
	}
}

// WithDatetimeMapping option creates a new field with a datetime mapping.
func WithDatetimeMapping(fieldname string) PropertiesOption {
	return func(mp Properties) {
		mp[fieldname] = types.NewDateNanosProperty()
	}
}

// WithTextMapping option creates a new field with a text mapping.
func WithTextMapping(fieldname string, settings *types.TextProperty) PropertiesOption {
	return func(mp Properties) {
		if settings == nil {
			mp[fieldname] = types.NewTextProperty()
		} else {
			mp[fieldname] = settings
		}
	}
}

// WithKeywordMapping option creates a new field with a keyword mapping.
func WithKeywordMapping(fieldname string) PropertiesOption {
	return func(mp Properties) {
		mp[fieldname] = types.NewKeywordProperty()
	}
}

// WithFlattenedMapping option creates a new field with a flattened mapping.
func WithFlattenedMapping(fieldname string) PropertiesOption {
	return func(mp Properties) {
		mp[fieldname] = types.NewFlattenedProperty()
	}
}

// WithBinaryMapping option creates a new field with a binary mapping.
func WithBinaryMapping(fieldname string) PropertiesOption {
	return func(mp Properties) {
		mp[fieldname] = types.NewBinaryProperty()
	}
}

// WithObjectMapping option creates a new field with a object mapping.
func WithObjectMapping(fieldname string, options ...PropertiesOption) PropertiesOption {
	return func(mp Properties) {
		mp[fieldname] = types.ObjectProperty{
			Properties: NewProperties(options...),
		}
	}
}

// ComponentTemplate represents a component template, a template that can be used within an index template.
type ComponentTemplate struct {
	*putcomponenttemplate.Request

	name string
}

// ComponentTemplateOption is a functional option to apply to a component template.
type ComponentTemplateOption Option[*ComponentTemplate]

// NewComponentTemplate creates a component template with the given name and template options.
func NewComponentTemplate(name string, templateSettings *Template, options ...ComponentTemplateOption) *ComponentTemplate {
	template := &ComponentTemplate{
		name: name,
		Request: &putcomponenttemplate.Request{
			Template: templateSettings.IndexState,
		},
	}
	for option := range slices.Values(options) {
		option(template)
	}
	return template
}

// WithComponentTemplateMetadata sets the given metadata in the component template.
func WithComponentTemplateMetadata(metadata types.Metadata) ComponentTemplateOption {
	return func(ct *ComponentTemplate) {
		ct.Meta_ = metadata
	}
}

// Put will send a request to create the component template in the cluster.
func (t *ComponentTemplate) Put(ctx context.Context, api *elasticsearch.TypedClient) error {
	_, err := api.Cluster.PutComponentTemplate(t.name).Request(t.Request).Do(ctx)
	if err != nil {
		return fmt.Errorf("put component template: %w", err)
	}

	return nil
}

// IndexTemplate represents an index template.
type IndexTemplate struct {
	*putindextemplate.Request

	name string
}

// IndexTemplateOption is a functional option to apply to an index template.
type IndexTemplateOption Option[*IndexTemplate]

// NewIndexTemplate creates an index template with the given name and template options.
func NewIndexTemplate(name string, options ...IndexTemplateOption) *IndexTemplate {
	template := &IndexTemplate{
		name:    name,
		Request: putindextemplate.NewRequest(),
	}
	for option := range slices.Values(options) {
		option(template)
	}
	return template
}

// WithComponentTemplates option sets the index template to be composed from the given component template names.
func WithComponentTemplates(names ...string) IndexTemplateOption {
	return func(it *IndexTemplate) {
		it.ComposedOf = names
	}
}

// WithIndexPatterns option sets the index template to match and apply to the given index patterns.
func WithIndexPatterns(patterns ...string) IndexTemplateOption {
	return func(it *IndexTemplate) {
		it.IndexPatterns = patterns
	}
}

// AsDatastream option will ensure the indicies using this template will be treated as a datastream.
func AsDatastream(value bool) IndexTemplateOption {
	return func(it *IndexTemplate) {
		if value {
			it.DataStream = types.NewDataStreamVisibility()
		}
	}
}

// WithIndexTemplateMetadata sets the given metadata in the index template.
func WithIndexTemplateMetadata(metadata types.Metadata) IndexTemplateOption {
	return func(ct *IndexTemplate) {
		ct.Meta_ = metadata
	}
}

// Put will send a request to create the index template in the cluster.
func (t *IndexTemplate) Put(ctx context.Context, api *elasticsearch.TypedClient) error {
	_, err := api.Indices.PutIndexTemplate(t.name).Request(t.Request).Do(ctx)
	if err != nil {
		return fmt.Errorf("put index template: %w", err)
	}

	return nil
}
