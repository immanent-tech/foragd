/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package opensearch

import (
	"strings"
	"time"
)

const (
	// indexWriteSuffix is the suffix appended to indices that are used for write (indexing) operations.
	indexWriteSuffix = "rw"
	// indexReadSuffix is the suffix appended to indices that are used for read (search, get) operations.
	indexReadSuffix = "ro"
)

// Index represents an index in OpenSearch.
type Index string

func (i Index) String() string {
	return string(i)
}

// IndexReadAlias returns the read alias for the given [Index].
func (c *Client) IndexReadAlias(name Index) string {
	return name.String() + "_" + c.environment + "_" + indexReadSuffix
}

// IndexWriteAlias returns the write alias for the given [Index].
func (c *Client) IndexWriteAlias(name Index) string {
	return name.String() + "_" + c.environment + "_" + indexWriteSuffix
}

// IndexCanonicalName returns a unique, time-stamped [Index] name that should be used for all underlying index management
// requests.
func (c *Client) IndexCanonicalName(name Index) string {
	return strings.Join(
		[]string{name.String(), c.environment, time.Now().Format("20060102150405"), "000000"},
		"-",
	)
}
