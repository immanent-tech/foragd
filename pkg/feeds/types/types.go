// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package types contains methods and objects that are shared across different Feed schemas/specifications.
package types

import (
	"encoding/xml"
)

var (
	// MimeTypesRSS contains canonical/standard mimetypes for RSS feeds.
	MimeTypesRSS = []string{"application/rss+xml", "application/rdf+xml"}
	// MimeTypesAtom contains canonical/standard mimetypes for Atom feeds.
	MimeTypesAtom = []string{"application/atom+xml"}
	// MimeTypesIndeterminate contains mimetypes that can be used for either RSS/Atom feeds and don't give any clues to
	// the actual type.
	MimeTypesIndeterminate = []string{"application/xml", "text/xml"}
)

// String will return the value of the object.
func (c *CustomTypeBase) String() string {
	return c.Value
}

// NewXMLAttr is a convienience function to create an xml.Attr from a name/value/namespace combination. The namespace
// value is optional, but the name and value should be provided.
func NewXMLAttr(name, value, namespace string) xml.Attr {
	return xml.Attr{
		Name: xml.Name{
			Space: namespace,
			Local: name,
		},
		Value: value,
	}
}
