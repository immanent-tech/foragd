// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package opengraph

import (
	"os"
	"slices"

	"github.com/immanent-tech/foragd/config"
)

// Metadata represents the default opengraph metadata properties used by the app on pages.
type Metadata struct {
	Title       Property   `validate:"required"`
	ObjectType  ObjectType `validate:"required"`
	URL         Property   `validate:"required"`
	Image       Property   `validate:"required"`
	Description Property
}

// NewMetadata creates a new opengraph Metadata object with properties set to values given by the options. Where a
// property is not set by an option, a default value will be used.
func NewMetadata(options ...Option) *Metadata {
	metadata := &Metadata{
		Title: Property{
			Value: config.AppName,
		},
		ObjectType: ObjectType{
			Value:      "website",
			Attributes: make(map[string]string),
		},
		URL: Property{
			Value: os.Getenv("FORAGD_BASEURL"),
		},
		Image: Property{
			Value: os.Getenv("FORAGD_BASEURL") + "/content/logo-color.webp",
		},
		Description: Property{
			Value: config.AppDescription,
		},
	}
	for option := range slices.Values(options) {
		option(metadata)
	}

	return metadata
}

type Property struct {
	Value string
}

type ObjectType struct {
	Value      string
	Attributes map[string]string
}

type Option func(*Metadata)

// WithTitle option sets a custom og:title property with optional element attributes. If this option is not used a
// default title will be set.
func WithTitle(title string) Option {
	return func(m *Metadata) {
		m.Title.Value = title
	}
}

// WithDescription option sets a custom og:desc property with optional element attributes. If this option is not used a
// default description will be set.
func WithDescription(desc string) Option {
	return func(m *Metadata) {
		m.Description.Value = desc
	}
}

// WithType option sets a custom og:type property with optional element attributes. If this option is not used a
// default type will be set.
func WithType(objectType string, attributes map[string]string) Option {
	return func(m *Metadata) {
		m.ObjectType.Value = objectType
		if attributes != nil {
			m.ObjectType.Attributes = attributes
		}
	}
}

// WithURL option sets a custom og:url property with optional element attributes. If this option is not used a
// default url will be set.
func WithURL(url string) Option {
	return func(m *Metadata) {
		m.URL.Value = url
	}
}

// WithImage option sets a custom og:image property with optional element attributes. If this option is not used a
// default image will be set.
func WithImage(image string) Option {
	return func(m *Metadata) {
		m.Image.Value = image
	}
}
