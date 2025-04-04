// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package types

import (
	"encoding/xml"
)

var (
	MimeTypesRSS           = []string{"application/rss+xml", "application/rdf+xml"}
	MimeTypesAtom          = []string{"application/atom+xml"}
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
