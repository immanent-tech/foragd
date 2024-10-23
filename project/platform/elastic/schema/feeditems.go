// Copyright (C) 2024 Joshua Rich <joshua.rich@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
			"externalID":  types.NewKeywordProperty(),
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
