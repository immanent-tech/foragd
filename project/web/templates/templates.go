// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"errors"

	"github.com/joshuar/go-feed-me/internal/models"
)

type (
	Option[T any]   func(T)
	ComponentOption Option[*Component]
)

var (
	ErrNewComponent     = errors.New("could not create component")
	ErrDisplayComponent = errors.New("could not display component")
)

// DisplayAs option defines how the Component should be displayed.
func DisplayAs(displayType ComponentType) ComponentOption {
	return func(c *Component) {
		c.DisplayType = displayType
	}
}

func WithRoute(label string, route *models.APIRoute) ComponentOption {
	return func(c *Component) {
		c.Routes[label] = route
	}
}

// NewComponent will create a new Component using the given object as a
// data-source and with the given options.
func NewComponent(object any, options ...ComponentOption) (*Component, error) {
	var err error

	component := &Component{
		Routes: make(map[string]*models.APIRoute),
	}

	switch data := object.(type) {
	case models.APIFeed:
		err = component.DataSource.FromFeed(data)
	case models.APIItem:
		err = component.DataSource.FromItem(data)
	}

	if err != nil {
		return nil, errors.Join(ErrNewComponent, err)
	}

	for _, option := range options {
		option(component)
	}

	return component, nil
}
