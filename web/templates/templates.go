// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import (
	"encoding/json"
)

func generateHXVals(values map[string]any) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}
