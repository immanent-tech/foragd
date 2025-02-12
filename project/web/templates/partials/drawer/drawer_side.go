// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package drawer

import (
	"github.com/a-h/templ"
	components "github.com/joshuar/go-templ-daisyui"
	"github.com/joshuar/go-templ-daisyui/actions/button"
	"github.com/joshuar/go-templ-daisyui/attributes"
)

type SideOption components.Option2[*SideProps]

type SideProps struct {
	ActionButtons  []*button.Props
	FilterControls []templ.Component
	*attributes.Attributes
}

func WithActionButtons(buttons ...*button.Props) SideOption {
	return func(p *SideProps) {
		p.ActionButtons = buttons
	}
}

func WithFilterControls(filters ...templ.Component) SideOption {
	return func(p *SideProps) {
		p.FilterControls = filters
	}
}

func WithID(id attributes.ID) SideOption {
	return func(p *SideProps) {
		p.SetID(id)
	}
}

func BuildSide(options ...SideOption) *SideProps {
	side := &SideProps{
		Attributes: attributes.New(),
	}

	for _, option := range options {
		option(side)
	}

	return side
}
