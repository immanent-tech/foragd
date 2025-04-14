// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import "time"

// ObjectMetadata contains methods for retrieving the metadata information about the Object.
type ObjectMetadata interface {
	GetTitle() string
	GetDescription() string
	GetLink() string
	GetPublishedDate() time.Time
	GetUpdatedDate() time.Time
}

// HasID contains methods for retrieving an Objects unique ID.
type HasID interface {
	GetID() string
}

// HasMedia contains methods for retrieving an Object's media, such as audio and video.
type HasMedia interface {
	GetImage() *Image
}

// MediaEditable indicates that the media of the object can be changed.
type MediaEditable interface {
	SetImage(image *Image)
}

// HasAttribution contains methods for retrieving values that relate to the copyright, rights, authors and
// contributors of an Object.
type HasAttribution interface {
	GetAuthors() []string
	GetContributors() []string
	GetRights() string
}

// HasContent contains methods for retrieving any embedded content of the Object.
type HasContent interface {
	GetContent() *Content
}

// HasTaxonomy contains methods for retrieving categorization and taxonomy values of an Object.
type HasTaxonomy interface {
	GetCategories() []string
}

// HasLocalization contains methods for retrieving localization information of an Object.
type HasLocalization interface {
	GetLanguage() string
}

// Source contains methods for retrieving or setting the source of the Object.
type Source interface {
	GetSourceURL() string
}

// SourceEditable indicates the source URL for the object can be changed.
type SourceEditable interface {
	SetSourceURL(url string)
}

// ObjectCommon contains all methods common across all objects.
type ObjectCommon interface {
	ObjectMetadata
	HasAttribution
	HasLocalization
	HasTaxonomy
	HasMedia
}

// ItemSource is an abstraction representing an individual Item from any type of Feed source.
type ItemSource interface {
	ObjectCommon
	HasID
	HasContent
}

// FeedSource is an abstraction representing any type of Feed.
type FeedSource interface {
	ObjectCommon
	Source
	SourceEditable
	MediaEditable
	GetItems() []ItemSource
}
