// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// FeedsIndexTemplate contains the mapping and setting component templates for
// the feeds index.
func FeedsIndexTemplate() IndexTemplate {
	return IndexTemplate{
		Name:          FeedSchemaPrefix,
		IndexPatterns: []string{FeedSchemaPrefix + "-*"},
		Components: []ComponentTemplate{
			{
				Name: FeedSchemaPrefix + "_mappings",
				Template: types.IndexState{
					Mappings: feedMapping(),
				},
			},
			{
				Name: FeedSchemaPrefix + "_settings",
				Template: types.IndexState{
					Aliases: map[string]types.Alias{
						"feeds": {},
					},
				},
			},
		},
		Priority: defaultPriority,
	}
}

// FeedItemsIndexTemplate contains the mapping and setting component templates for
// the feed-items index.
func FeeditemsIndexTemplate() IndexTemplate {
	lifecycleName := FeedItemsSchemaPrefix
	lifecycle := types.NewIndexSettingsLifecycle()
	lifecycle.Name = &lifecycleName

	return IndexTemplate{
		Name:          FeedItemsSchemaPrefix,
		IndexPatterns: []string{FeedItemsSchemaPrefix + "-*"},
		Components: []ComponentTemplate{
			{
				Name: FeedItemsSchemaPrefix + "_mappings",
				Template: types.IndexState{
					Mappings: feeditemsMapping(),
				},
			},
			{
				Name: FeedItemsSchemaPrefix + "_settings",
				Template: types.IndexState{
					Settings: &types.IndexSettings{
						Lifecycle: lifecycle,
					},
				},
			},
		},
		DataStream: types.NewDataStreamVisibility(),
		Priority:   defaultPriority,
	}
}

// ReadItemsIndexTemplate contains the mapping and setting component templates for
// the read-items index.
func ReadItemsIndexTemplate() IndexTemplate {
	lifecycleName := ReadItemsSchemaPrefix
	lifecycle := types.NewIndexSettingsLifecycle()
	lifecycle.Name = &lifecycleName

	return IndexTemplate{
		Name:          ReadItemsSchemaPrefix,
		IndexPatterns: []string{ReadItemsSchemaPrefix + "-*"},
		Components: []ComponentTemplate{
			{
				Name: ReadItemsSchemaPrefix + "_mappings",
				Template: types.IndexState{
					Mappings: readItemsMapping(),
				},
			},
			{
				Name: ReadItemsSchemaPrefix + "_settings",
				Template: types.IndexState{
					Settings: &types.IndexSettings{
						Lifecycle: lifecycle,
					},
				},
			},
		},
		DataStream: types.NewDataStreamVisibility(),
		Priority:   defaultPriority,
	}
}
