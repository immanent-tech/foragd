// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package validation

import (
	"html"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

var safePrinter = bluemonday.UGCPolicy()

// SanitizeString attempts to "sanitize" a string value. It will strip any leading/trailing whitespace and then run the
// string through bluemonday to remove dangerous components. This should retain HTML5 content.
func SanitizeString(str string) string {
	return html.UnescapeString(safePrinter.Sanitize(strings.TrimSpace(str)))
}
