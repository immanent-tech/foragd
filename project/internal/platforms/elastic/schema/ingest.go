// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import (
	"github.com/elastic/go-elasticsearch/v8/typedapi/ingest/putpipeline"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

const (
	IngestPipelineID = "gofeed"
)

func FeedItemsIngestPipeline() putpipeline.Request {
	// useUpdatedAsTimestampDesc := "Use updated date as @timestamp if not nil"
	// usePublishedAsTimestampDesc := "Use published date as @timestamp if not nil"
	// publishedParsedNotNull := "ctx?.updatedParsed != null"
	// publishedParsedNull := "ctx?.updatedParsed == null"
	// targetField := "@timestamp"
	removeIgnoreMissing := true
	removeDesc := "Remove deprecated and unneeded fields"

	return putpipeline.Request{
		Processors: []types.ProcessorContainer{
			// {
			// 	Date: &types.DateProcessor{
			// 		Field:       "updatedParsed",
			// 		TargetField: &targetField,
			// 		Formats:     []string{"strict_date_optional_time_nanos", "epoch_millis"},
			// 		If:          &publishedParsedNull,
			// 		Description: &useUpdatedAsTimestampDesc,
			// 	},
			// },
			// {
			// 	Date: &types.DateProcessor{
			// 		Field:       "publishedParsed",
			// 		TargetField: &targetField,
			// 		Formats:     []string{"strict_date_optional_time_nanos", "epoch_millis"},
			// 		If:          &publishedParsedNotNull,
			// 		Description: &usePublishedAsTimestampDesc,
			// 	},
			// },
			{
				Remove: &types.RemoveProcessor{
					Field:         []string{"author", "published", "updated", "items"},
					IgnoreMissing: &removeIgnoreMissing,
					Description:   &removeDesc,
				},
			},
		},
	}
}
