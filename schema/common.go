/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package schema

import (
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"

	"github.com/immanent-tech/foragd/providers/opensearch/indices"
)

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

// CommonMappings are field mappings common across [models.Item] and [models.Feed].
var CommonMappings = indices.NewMappings(
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("published"),
	indices.WithMapping[opensearchapi.CommonMappingDateNanosProperty]("updated"),
	indices.WithMapping("title",
		indices.WithTextMappingProperties(
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("description", indices.WithTextMappingAnalyzer(htmlAnalyzer)),
	indices.WithMapping("authors",
		indices.WithTextMappingAnalyzer(personAnalyzer),
		indices.WithTextMappingProperties(
			indices.WithMapping("raw", indices.WithKeywordIgnoreAbove(256)),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("contributors",
		indices.WithTextMappingAnalyzer(personAnalyzer),
		indices.WithTextMappingProperties(
			indices.WithMapping("raw", indices.WithKeywordIgnoreAbove(256)),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("categories",
		indices.WithTextMappingProperties(
			indices.WithMapping("raw",
				indices.WithKeywordIgnoreAbove(256),
			),
			indices.WithMapping("raw_nocase",
				indices.WithKeywordMappingNormalizer(lowercaseNormalizer),
				indices.WithKeywordIgnoreAbove(256),
			),
			indices.WithMapping("exact",
				indices.WithTextMappingAnalyzer(englishExactAnalyzer),
			),
			indices.WithMapping[opensearchapi.CommonMappingSearchAsYouTypeProperty]("search"),
		),
	),
	indices.WithMapping("language", indices.WithKeywordIgnoreAbove(24)),
	indices.WithMapping[opensearchapi.CommonMappingTextProperty]("copyright"),
	indices.WithMapping("source_type", indices.WithKeywordIgnoreAbove(256)),
	indices.WithMapping("url", indices.WithKeywordIgnoreAbove(256)),
	ImageMapping,
)

// CommonComponentTemplate is a component template containing the common mappings.
var CommonComponentTemplate = indices.NewComponentTemplateRequest("common",
	indices.WithIndexSettings(
		indices.WithMappings(CommonMappings),
		indices.WithAnalyzer(englishExactAnalyzer),
		indices.WithAnalyzer(personAnalyzer),
		indices.WithAnalyzer(htmlAnalyzer),
		indices.WithTokenizer(emailTokenizer),
		indices.WithNormalizer(lowercaseNormalizer),
	),
)

// ImageMapping is a common mapping configuration for an image.
var ImageMapping = indices.WithMapping("image",
	indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
	indices.WithObjectMappingProperties(
		indices.WithMapping("url", indices.WithKeywordIgnoreAbove(256)),
		indices.WithMapping[opensearchapi.CommonMappingTextProperty]("title"),
	),
)

// ArticleFiltersMapping is a common mapping configuration for article filtering fields.
var ArticleFiltersMapping = indices.WithMapping("article_filters",
	indices.WithObjectMappingDynamic(opensearchapi.NewCommonMappingDynamicFromBool(false)),
	indices.WithObjectMappingProperties(
		indices.WithMapping("text",
			indices.WithKeywordIgnoreAbove(512),
			indices.WithKeywordDocValues(false),
			indices.WithKeywordIndex(false),
		),
		indices.WithMapping("categories",
			indices.WithKeywordIgnoreAbove(512),
			indices.WithKeywordDocValues(false),
			indices.WithKeywordIndex(false),
		),
		indices.WithMapping("authors",
			indices.WithKeywordIgnoreAbove(512),
			indices.WithKeywordDocValues(false),
			indices.WithKeywordIndex(false),
		),
	),
)
