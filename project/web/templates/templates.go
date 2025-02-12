// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"errors"
	"maps"

	"github.com/a-h/templ"

	"github.com/joshuar/go-feed-me/internal/models"
)

type (
	Option[T any]   func(T)
	ComponentOption Option[*Component]
	ActionOption    Option[*Action]
)

var (
	ErrNewComponent     = errors.New("could not create component")
	ErrDisplayComponent = errors.New("could not display component")
)

// SetAttribute will set the attribute with the given key to the given value.
func (c *Component) SetAttribute(key string, value any) {
	if _, found := c.Attributes[key]; !found {
		c.Attributes[key] = value
	}
}

// UnsetAttribute will unset the attribute with the given key.
func (c *Component) UnsetAttribute(key string) {
	delete(c.Attributes, key)
}

// AddAttributes will ensure the component has the given attributes. Any
// existing attributes are merged with the given set of attributes.
func (c *Component) AddAttributes(attrs templ.Attributes) {
	if c.Attributes != nil {
		maps.Copy(c.Attributes, attrs)
	} else {
		c.Attributes = attrs
	}
}

// WithAtttributes option sets the given attributes on the Component.
func WithAttributes(attributes templ.Attributes) ComponentOption {
	return func(c *Component) {
		c.Attributes = attributes
	}
}

// WithActions are any actions the Component should have. How the Component will
// display the actions is Component-specific.
func WithActions(actions ...Action) ComponentOption {
	return func(c *Component) {
		c.Actions = actions
	}
}

// DisplayAs option defines how the Component should be displayed.
func DisplayAs(displayType DisplayType) ComponentOption {
	return func(c *Component) {
		c.DisplayType = displayType
	}
}

// NewComponent will create a new Component using the given object as a
// data-source and with the given options.
func NewComponent(object any, options ...ComponentOption) (*Component, error) {
	var err error

	component := &Component{
		Attributes: make(templ.Attributes),
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

// WithAtttributes option sets the given attributes on the Component.
func WithActionAttributes(attributes templ.Attributes) ActionOption {
	return func(ca *Action) {
		ca.Attributes = attributes
	}
}

// WithActionIcon option defines an icon that should be displayed on or with the Action.
func WithActionIcon(icon string) ActionOption {
	return func(ca *Action) {
		ca.Icon = &icon
	}
}

// NewAction will create a new Action with the given options.
func NewAction(label string, options ...ActionOption) Action {
	component := &Action{
		Label:      label,
		Attributes: make(templ.Attributes),
	}

	for _, option := range options {
		option(component)
	}

	return *component
}
