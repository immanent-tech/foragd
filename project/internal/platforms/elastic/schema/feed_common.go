// Copyright 2024 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package schema

import "github.com/elastic/go-elasticsearch/v8/typedapi/types"

const (
	FeedSchemaPrefix      = "feeds"
	FeedItemsSchemaPrefix = "feeditems"
	ReadItemsSchemaPrefix = "readitems"
)

const (
	defaultPriority = 500
	timeStampField  = "updatedParsed"
)

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
