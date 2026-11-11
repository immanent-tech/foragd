/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch"
	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

const (
	FeedsIndex         opensearch.Index = "feeds"
	ItemsIndex         opensearch.Index = "items"
	FavoritesIndex     opensearch.Index = "favorites"
	UsersIndex         opensearch.Index = "users"
	SubscriptionsIndex opensearch.Index = "subscriptions"
	SchedulerIndex     opensearch.Index = "scheduler"
	SessionsIndex      opensearch.Index = "sessions"
	ImportsIndex       opensearch.Index = "imports"
)

var AllIndices = []opensearch.Index{
	FeedsIndex,
	ItemsIndex,
	FavoritesIndex,
	UsersIndex,
	SubscriptionsIndex,
	SchedulerIndex,
	SessionsIndex,
	ImportsIndex,
}

var (
	// englishExactAnalyzer provides exact (case-sensitive) matching.
	englishExactAnalyzer = indices.CustomAnalyzer{
		Name: "english_exact",
		Definition: opensearchapi.NewCommonAnalysisAnalyzerFromCustomAnalyzer(
			opensearchapi.CommonAnalysisCustomAnalyzer{
				Type:      "custom",
				Tokenizer: "standard",
				Filter:    []string{"lowercase"},
			},
		),
	}

	// emailTokenizer tokenizes text including emails and URLs.
	emailTokenizer = indices.CustomTokenizer{
		Name: "email_tokenizer",
		Definition: opensearchapi.NewCommonAnalysisTokenizerFromTokenizerDefinition(
			opensearchapi.NewCommonAnalysisTokenizerDefinitionFromUAXEmailURLTokenizer(
				opensearchapi.CommonAnalysisUAXEmailURLTokenizer{},
			),
		),
	}

	// personAnalyzer recognizes email addresses for people.
	personAnalyzer = indices.CustomAnalyzer{
		Name: "author_contributor",
		Definition: opensearchapi.NewCommonAnalysisAnalyzerFromCustomAnalyzer(
			opensearchapi.CommonAnalysisCustomAnalyzer{
				Type:      "custom",
				Tokenizer: emailTokenizer.Name,
				Filter:    []string{"lowercase", "stop"},
			},
		),
	}

	// htmlAnalyzer is the standard analyzer with HTML stripped/cleaned.
	htmlAnalyzer = indices.CustomAnalyzer{
		Name: "html",
		Definition: opensearchapi.NewCommonAnalysisAnalyzerFromCustomAnalyzer(
			opensearchapi.CommonAnalysisCustomAnalyzer{
				Type:       "custom",
				Tokenizer:  emailTokenizer.Name,
				Filter:     []string{"lowercase", "stop"},
				CharFilter: []string{"html_strip"},
			},
		),
	}

	// domainTokenizer tokenizes (splits) domain names.
	domainTokenizer = indices.CustomTokenizer{
		Name: "domain_tokenizer",
		Definition: opensearchapi.NewCommonAnalysisTokenizerFromTokenizerDefinition(
			opensearchapi.NewCommonAnalysisTokenizerDefinitionFromCharGroupTokenizer(
				opensearchapi.CommonAnalysisCharGroupTokenizer{
					TokenizeOnChars: []string{"."},
				},
			),
		),
	}

	// domainNameAnalyzer is an analyzer for domain names.
	domainNameAnalyzer = indices.CustomAnalyzer{
		Name: "domain",
		Definition: opensearchapi.NewCommonAnalysisAnalyzerFromCustomAnalyzer(
			opensearchapi.CommonAnalysisCustomAnalyzer{
				Type:      "custom",
				Tokenizer: domainTokenizer.Name,
				Filter:    []string{"lowercase"},
			},
		),
	}

	// lowercaseNormalizer converts keywords to lowercase.
	lowercaseNormalizer = indices.CustomNormalizer{
		Name: "keyword_lowercase",
		Definition: opensearchapi.NewCommonAnalysisNormalizerFromCustomNormalizer(
			opensearchapi.CommonAnalysisCustomNormalizer{
				Type:   "keyword",
				Filter: []string{"lowercase"},
			},
		),
	}
)
