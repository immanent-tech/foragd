// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"encoding/json"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/dynamicmapping"
)

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

func FeedItemsILMPolicy() *types.IlmPolicy {
	hotRolloverSize := "50gb"
	warmShrinkShards := 1
	warmForceMergeSegments := 1
	deleteMinAge := types.Duration("735d")

	return &types.IlmPolicy{
		Meta_: types.Metadata{
			"version":     json.RawMessage(`"v0.0.1"`),
			"description": json.RawMessage(`"ILM policy for read items"`),
		},
		Phases: types.Phases{
			Hot: &types.Phase{
				Actions: &types.IlmActions{
					Rollover: &types.RolloverAction{
						MaxSize: hotRolloverSize,
					},
				},
			},
			Warm: &types.Phase{
				Actions: &types.IlmActions{
					Shrink: &types.ShrinkAction{
						NumberOfShards: &warmShrinkShards,
					},
					Forcemerge: &types.ForceMergeAction{
						MaxNumSegments: warmForceMergeSegments,
					},
				},
			},
			Delete: &types.Phase{
				MinAge: &deleteMinAge,
				Actions: &types.IlmActions{
					Delete: types.NewDeleteAction(),
				},
			},
		},
	}
}

func readItemsMapping() *types.TypeMapping {
	return &types.TypeMapping{
		Dynamic: &dynamicmapping.False, // Ignore any additional fields in documents not listed in this mapping.
		Meta_: types.Metadata{
			"version":     json.RawMessage(`"v0.0.1"`),
			"description": json.RawMessage(`"Field mappings for read items"`),
		},
		Properties: map[string]types.Property{
			"@timestamp": types.NewDateNanosProperty(),
			"feed_id":    types.NewKeywordProperty(),
			"item_id":    types.NewKeywordProperty(),
			"user_id":    types.NewKeywordProperty(),
		},
	}
}

func ReadItemsILMPolicy() *types.IlmPolicy {
	hotRolloverSize := "50gb"
	warmShrinkShards := 1
	warmForceMergeSegments := 1
	deleteMinAge := types.Duration("735d")

	return &types.IlmPolicy{
		Meta_: types.Metadata{
			"version":     json.RawMessage(`"v0.0.1"`),
			"description": json.RawMessage(`"ILM policy for items"`),
		},
		Phases: types.Phases{
			Hot: &types.Phase{
				Actions: &types.IlmActions{
					Rollover: &types.RolloverAction{
						MaxSize: hotRolloverSize,
					},
				},
			},
			Warm: &types.Phase{
				Actions: &types.IlmActions{
					Shrink: &types.ShrinkAction{
						NumberOfShards: &warmShrinkShards,
					},
					Forcemerge: &types.ForceMergeAction{
						MaxNumSegments: warmForceMergeSegments,
					},
				},
			},
			Delete: &types.Phase{
				MinAge: &deleteMinAge,
				Actions: &types.IlmActions{
					Delete: types.NewDeleteAction(),
				},
			},
		},
	}
}
