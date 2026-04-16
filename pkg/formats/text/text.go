// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package text

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

const (
	// wpm is the words per minute average for English.
	//
	// https://en.wikipedia.org/wiki/Words_per_minute#Reading_and_comprehension
	wpm int = 228
)

// CountWords performs a rough count of the words in the given string.
func CountWords(text string) int {
	return len(splitToWords(text))
}

func ReadingTime(text string) time.Duration {
	words := splitToWords(text)
	minutes := float64(len(words)) / float64(wpm)
	dur, _ := time.ParseDuration(fmt.Sprintf("%.2fm", minutes))
	return dur
}

func splitToWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
}
