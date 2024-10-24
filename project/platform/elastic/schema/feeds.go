// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const (
	FeedItemsSchemaID   = "feeditem"
	feeditemsMappingsID = FeedItemsSchemaID + "_mappings"
	feeditemsSettingsID = FeedItemsSchemaID + "_settings"
)

const (
	defaultPriority = 500
	timeStampField  = "updatedParsed"
)

func FeedItemsIngestPipeline() putpipeline.Request {
	publishedParsedNotNull := "ctx?.updatedParsed != null"
	publishedParsedNull := "ctx?.updatedParsed == null"
	targetField := "@timestamp"

	return putpipeline.Request{
		Processors: []types.ProcessorContainer{
			{
				Date: &types.DateProcessor{
					Field:       "updatedParsed",
					TargetField: &targetField,
					Formats:     []string{"strict_date_optional_time_nanos", "epoch_millis"},
					If:          &publishedParsedNotNull,
				},
			},
			{
				Date: &types.DateProcessor{
					Field:       "publishedParsed",
					TargetField: &targetField,
					Formats:     []string{"strict_date_optional_time_nanos", "epoch_millis"},
					If:          &publishedParsedNull,
				},
			},
		},
	}
}

func FeeditemsIndexTemplate() IndexTemplate {
	return IndexTemplate{
		Name:          FeedItemsSchemaID,
		IndexPatterns: []string{"feeditems*"},
		Components:    []ComponentTemplate{feeditemsMappingsComponent(), feeditemsSettingsComponent(FeedItemsSchemaID)},
		Priority:      defaultPriority,
	}
}

func feeditemsMappingsComponent() ComponentTemplate {
	return ComponentTemplate{
		Name: feeditemsMappingsID,
		Template: types.IndexState{
			Mappings: feeditemsMapping(),
		},
	}
}

func feeditemsSettingsComponent(ilmPolicyName string) ComponentTemplate {
	return ComponentTemplate{
		Name: feeditemsSettingsID,
		Template: types.IndexState{
			Settings: &types.IndexSettings{
				Lifecycle: &types.IndexSettingsLifecycle{
					Name: &ilmPolicyName,
				},
			},
		},
	}
}

func feeditemsMapping() *types.TypeMapping {
	return &types.TypeMapping{
		Properties: map[string]types.Property{
			"@timestamp":  types.NewDateNanosProperty(),
			"feedID":      types.NewKeywordProperty(),
			"title":       defaultTextFieldMapping(),
			"description": types.NewTextProperty(), // ? additional config required
			"content":     types.NewTextProperty(), // ? additional config required
			// links can be array.
			"links":           types.NewKeywordProperty(), // ? define analyzer
			"updated":         types.NewTextProperty(),
			"updatedParsed":   types.NewDateNanosProperty(),
			"published":       types.NewTextProperty(),
			"publishedParsed": types.NewDateNanosProperty(),
			"author_names":    defaultTextFieldMapping(),
			"author_emails":   types.NewKeywordProperty(),
			// authors can be an array.
			"authors": types.ObjectProperty{
				Properties: map[string]types.Property{
					"name": types.TextProperty{
						CopyTo: []string{"author_names"},
					},
					"email": types.KeywordProperty{
						CopyTo: []string{"author_emails"},
					},
				},
			},
			"guid": types.NewKeywordProperty(),
			"image": types.ObjectProperty{
				Properties: map[string]types.Property{
					"URL":   types.NewKeywordProperty(),
					"Title": defaultTextFieldMapping(),
				},
			},
			// categories can be array.
			"categories": defaultTextFieldMapping(),
			// enclosures can be array.
			"enclosures": types.ObjectProperty{
				Properties: map[string]types.Property{
					"url":    types.NewKeywordProperty(),
					"length": types.NewKeywordProperty(),
					"type":   types.NewKeywordProperty(),
				},
			},
			"dublincoreext": dublinCoreMapping(),
			"itunesext":     iTunesItemMapping(),
			"extensions":    types.NewFlattenedProperty(),
			"custom":        types.NewFlattenedProperty(),
		},
	}
}

func dublinCoreMapping() types.ObjectProperty {
	return types.ObjectProperty{
		Properties: map[string]types.Property{
			"title":       defaultTextFieldMapping(),
			"creator":     defaultTextFieldMapping(),
			"author":      defaultTextFieldMapping(),
			"subject":     defaultTextFieldMapping(),
			"description": defaultTextFieldMapping(),
			"publisher":   defaultTextFieldMapping(),
			"contributor": defaultTextFieldMapping(),
			"date":        defaultTextFieldMapping(),
			"type":        defaultTextFieldMapping(),
			"format":      defaultTextFieldMapping(),
			"identifier":  defaultTextFieldMapping(),
			"source":      defaultTextFieldMapping(),
			"language":    defaultTextFieldMapping(),
			"relation":    defaultTextFieldMapping(),
			"coverage":    defaultTextFieldMapping(),
			"rights":      defaultTextFieldMapping(),
		},
	}
}

func iTunesItemMapping() types.ObjectProperty {
	return types.ObjectProperty{
		Properties: map[string]types.Property{
			"author":            defaultTextFieldMapping(),
			"block":             defaultTextFieldMapping(),
			"duration":          defaultTextFieldMapping(),
			"explicit":          defaultTextFieldMapping(),
			"keywords":          defaultTextFieldMapping(),
			"subtitle":          defaultTextFieldMapping(),
			"summary":           defaultTextFieldMapping(),
			"image":             defaultTextFieldMapping(),
			"isClosedCaptioned": defaultTextFieldMapping(),
			"episode":           defaultTextFieldMapping(),
			"season":            defaultTextFieldMapping(),
			"order":             defaultTextFieldMapping(),
			"episodeType":       defaultTextFieldMapping(),
		},
	}
}

// defaultTextFieldMapping is an elasticsearch field mapping for text fields that
// maps the context as both text and keyword types.
func defaultTextFieldMapping() types.TextProperty {
	return types.TextProperty{
		Type: "text",
		Fields: map[string]types.Property{
			"raw": types.NewKeywordProperty(),
		},
	}
}
