// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

func FeedsIndexTemplate() IndexTemplate {
	return IndexTemplate{
		Name:          FeedSchemaPrefix,
		IndexPatterns: []string{"feeds-*"},
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

func FeeditemsIndexTemplate() IndexTemplate {
	lifecycleName := FeedItemsSchemaPrefix
	lifecycle := types.NewIndexSettingsLifecycle()
	lifecycle.Name = &lifecycleName

	return IndexTemplate{
		Name:          FeedItemsSchemaPrefix,
		IndexPatterns: []string{"feeditems-*"},
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
