/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package templates

import (
	"hash/fnv"
	"strings"
	"unicode"
)

func articleURL(id string) string { return "/articles/" + id }

// / initialLetter is the letter shown on the placeholder thumbnail.
func initialLetter(text string) string {
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return strings.ToUpper(string(r))
		}
	}
	return "#"
}

// hueFromText generates a hue (0-359) from the given text. Using the same text should generate a stable hue color.
func hueFromText(text string) int {
	h := fnv.New32a()
	h.Write([]byte(text))
	return int(h.Sum32() % 360)
}
