// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package dc

import "github.com/joshuar/go-feed-me/pkg/feeds/types"

func (c *DCCreator) String() string {
	return types.SanitizeString(c.Value)
}

func (c *DCContributor) String() string {
	return types.SanitizeString(c.Value)
}

func (c *DCTitle) String() string {
	return types.SanitizeString(c.Value)
}

func (c *DCDescription) String() string {
	return types.SanitizeString(c.Value)
}
