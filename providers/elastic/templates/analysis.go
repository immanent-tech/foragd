// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package templates

import "github.com/elastic/go-elasticsearch/v9/typedapi/types"

// CustomAnalyzer holds the name and definition of a custom analyzer.
type CustomAnalyser struct {
	Name       string
	Definition types.CustomAnalyzer
}

// CustomNormalizer holds the name and definition of a custom normalizer.
type CustomNormalizer struct {
	Name       string
	Definition types.CustomNormalizer
}

// CustomTokenizer holds the name and definition of a custom tokenizer.
type CustomTokenizer struct {
	Name       string
	Definition types.Tokenizer
}
