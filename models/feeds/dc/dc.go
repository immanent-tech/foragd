// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

// Package dc contains objects and methods defining the Dublin Core extension used by RSS/Atom syndication formats.
package dc

import "github.com/joshuar/go-feed-me/models/feeds/sanitization"

func (c *DCCreator) String() string {
	return sanitization.SanitizeString(c.Value)
}

func (c *DCContributor) String() string {
	return sanitization.SanitizeString(c.Value)
}

func (c *DCTitle) String() string {
	return sanitization.SanitizeString(c.Value)
}

func (c *DCDescription) String() string {
	return sanitization.SanitizeString(c.Value)
}

func (c *DCRights) String() string {
	return sanitization.SanitizeString(c.Value)
}

func (c *DCLanguage) String() string {
	return sanitization.SanitizeString(c.Value)
}
