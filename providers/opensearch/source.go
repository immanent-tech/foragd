/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package opensearch

import (
	"encoding/json"
	"fmt"
)

// ExtractSource extracts the `_source` field from the raw response.
func ExtractSource[T any](doc json.RawMessage) (T, error) {
	var source T
	if err := json.Unmarshal(doc, &source); err != nil {
		return source, fmt.Errorf("%w: %w", ErrExtractSource, err)
	}
	return source, nil
}
