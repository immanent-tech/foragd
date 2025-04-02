// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package atom

import (
	"fmt"
)

// String returns string-ified format of the PersonConstruct. This will be the format "name (email)". The email part is
// omitted if the PersonConstruct has no email.
func (p *PersonConstruct) String() string {
	if p.Email != nil {
		return fmt.Sprintf("%s (%s)", p.Name.Value, p.Email)
	}
	return p.Name.Value
}

// String returns the string-ified format of the Category. It will return the first found of: any human-readable label,
// the element value or the term attribute value, in that order.
func (c *Category) String() string {
	// Use the label attribute if present.
	if c.Label != nil && c.Label.Value != "" {
		return c.Label.Value
	}
	// Use any value if present.
	if c.Value != nil && *c.Value != "" {
		return *c.Value
	}
	// Use the term attribute.
	return c.Term.Value
}

func (t *Title) String() string {
	return t.Value
}

func (t *Subtitle) String() string {
	return t.Value
}

func (s *Summary) String() string {
	return s.Value
}
