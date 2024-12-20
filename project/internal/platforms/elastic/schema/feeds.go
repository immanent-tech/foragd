// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/dynamicmapping"
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
