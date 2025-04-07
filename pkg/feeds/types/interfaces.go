// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import "time"

// ObjectID contains methods for retrieving an Objects unique ID.
type ObjectID interface {
	GetID() string
}

// ObjectMedia contains methods for retrieving an Object's media, such as audio and video.
type ObjectMedia interface {
	GetImage() *Image
}

// ObjectMetadata contains methods for retrieving the metadata information about the Object.
type ObjectMetadata interface {
	GetTitle() string
	GetDescription() string
	GetLink() string
	GetPublishedDate() time.Time
	GetUpdatedDate() time.Time
}

// ObjectAttribution contains methods for retrieving values that relate to the copyright, rights, authors and
// contributors of an Object.
type ObjectAttribution interface {
	GetAuthors() []string
	GetContributors() []string
	GetRights() string
}

// ObjectContent contains methods for retrieving any embedded content of the Object.
type ObjectContent interface {
	GetContent() *Content
}

// ObjectTaxonomy contains methods for retrieving categorization and taxonomy values of an Object.
type ObjectTaxonomy interface {
	GetCategories() []Category
}

// ObjectLocalization contains methods for retrieving localization information of an Object.
type ObjectLocalization interface {
	GetLanguage() string
}

// ObjectSource contains methods for retrieving or setting the source of the Object.
type ObjectSource interface {
	GetSourceURL() string
	SetSourceURL(url string)
}

// ItemSource is an abstraction representing an individual Item from any type of Feed source.
type ItemSource interface {
	ObjectMetadata
	ObjectAttribution
	ObjectLocalization
	ObjectContent
	ObjectTaxonomy
	ObjectID
	ObjectMedia
}

// FeedSource is an abstraction representing any type of Feed.
type FeedSource interface {
	ObjectMetadata
	ObjectSource
	ObjectAttribution
	ObjectLocalization
	ObjectTaxonomy
	ObjectMedia
	GetItems() []ItemSource
}
