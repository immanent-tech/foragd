// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/dynamicmapping"
)

const (
	FeedSchemaPrefix      = "feeds"
	FeedItemsSchemaPrefix = "feeditems"
)

const (
	defaultPriority = 500
	timeStampField  = "updatedParsed"
)

// feedMapping defines the Elasticsearch field mapping for feeds-* indices.
func feedMapping() *types.TypeMapping {
	return &types.TypeMapping{
		Meta_: types.Metadata{
			"version":     json.RawMessage(`"v0.0.1"`),
			"description": json.RawMessage(`"Field mappings for feeds"`),
		},
		Dynamic: &dynamicmapping.False, // Ignore any additional fields in documents not listed in this mapping.
		Properties: map[string]types.Property{
			"@timestamp":      types.NewDateNanosProperty(),
			"feed_id":         types.NewKeywordProperty(),
			"created_at":      types.NewDateNanosProperty(),
			"title":           defaultTextFieldMapping(),
			"description":     types.NewTextProperty(), // ? additional config required
			"content":         types.NewTextProperty(), // ? additional config required
			"link":            types.NewKeywordProperty(),
			"feedLink":        types.NewKeywordProperty(),
			"links":           types.NewKeywordProperty(),
			"feedType":        types.NewKeywordProperty(),
			"feedVersion":     types.NewKeywordProperty(),
			"updatedParsed":   types.NewDateNanosProperty(),
			"publishedParsed": types.NewDateNanosProperty(),
			// authors can be an array.
			"authors": types.ObjectProperty{
				Properties: map[string]types.Property{
					"name":  defaultTextFieldMapping(),
					"email": defaultTextFieldMapping(),
				},
			},
			"language": defaultTextFieldMapping(),
			"image": types.ObjectProperty{
				Properties: map[string]types.Property{
					"url":   types.NewKeywordProperty(),
					"title": defaultTextFieldMapping(),
				},
			},
			"copyright": defaultTextFieldMapping(),
			"generator": defaultTextFieldMapping(),

			// categories can be array.
			"categories": defaultTextFieldMapping(),
			// enclosures can be array.
			"dublincoreext": dublinCoreMapping(),
			"itunesext":     iTunesItemMapping(),
			"extensions":    types.NewFlattenedProperty(),
			"custom":        types.NewFlattenedProperty(),
		},
	}
}

func feeditemsMapping() *types.TypeMapping {
	return &types.TypeMapping{
		Dynamic: &dynamicmapping.False, // Ignore any additional fields in documents not listed in this mapping.
		Meta_: types.Metadata{
			"version":     json.RawMessage(`"v0.0.1"`),
			"description": json.RawMessage(`"Field mappings for feed items"`),
		},
		Properties: map[string]types.Property{
			"@timestamp":  types.NewDateNanosProperty(),
			"feed_id":     types.NewKeywordProperty(),
			"item_id":     types.NewKeywordProperty(),
			"title":       defaultTextFieldMapping(),
			"description": types.NewTextProperty(), // ? additional config required
			"content":     types.NewTextProperty(), // ? additional config required
			// links can be array.
			"links":           types.NewKeywordProperty(), // ? define analyzer
			"updatedParsed":   types.NewDateNanosProperty(),
			"publishedParsed": types.NewDateNanosProperty(),
			// authors can be an array.
			"authors": types.ObjectProperty{
				Properties: map[string]types.Property{
					"name":  defaultTextFieldMapping(),
					"email": defaultTextFieldMapping(),
				},
			},
			"guid": types.NewKeywordProperty(),
			"image": types.ObjectProperty{
				Properties: map[string]types.Property{
					"url":   types.NewKeywordProperty(),
					"title": defaultTextFieldMapping(),
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
