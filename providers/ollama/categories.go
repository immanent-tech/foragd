// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package ollama

import (
	"cmp"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"sync"
)

type CategoryEmbedding struct {
	Label       string
	Description string
	Embedding   []float32
}

var (
	embedOnce  sync.Once
	embedCache []CategoryEmbedding
	embedErr   error
)

var buildIABCategoryEmbeddings = sync.OnceValues(func() ([]CategoryEmbedding, error) {
	embedCache = iabTier1Categories
	embedOnce.Do(func() {
		embedErr = BuildCategories(embedCache)
	})
	if embedErr != nil {
		slog.Error("Failed to build IAB category embeddings.",
			slog.Any("error", embedErr))
		// Reset so the next call retries instead of returning a stale error forever.
		embedOnce = sync.Once{}
	} else {
		slog.Debug("Built embeddings for IAB categories.")
	}
	return embedCache, embedErr
})

var buildBasePriors = sync.OnceValues(func() (*DomainCategoryPrior, error) {
	return BuildPriors(), nil
})

type Category struct {
	Label string
	Score float32
}

// Classify embeds content and returns the best-matching category label along with its cosine similarity score
// (confidence proxy).
func Classify(
	content, contentURL string,
	categories []CategoryEmbedding,
	domainPriors *DomainCategoryPrior,
	alpha float32,
) ([]Category, error) {
	if len(categories) == 0 {
		var err error
		categories, err = buildIABCategoryEmbeddings()
		if err != nil {
			return nil, fmt.Errorf("build categories: %w", err)
		}
	}
	if domainPriors == nil {
		domainPriors, _ = buildBasePriors()
	}
	if err := LoadConfig(); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	signals, err := ExtractURLSignals(contentURL)
	if err != nil {
		return nil, err
	}

	embedText := content
	if len(signals.PathSegments) > 0 {
		n := min(len(signals.PathSegments), 3)
		hint := strings.Join(signals.PathSegments[:n], ", ")
		embedText = fmt.Sprintf("Section: %s. %s", hint, content)
	}

	vectors, err := EmbedBatch(embedText)
	if err != nil {
		return nil, fmt.Errorf("embedding content: %w", err)
	}
	contentVec := vectors[0]
	prior := domainPriors.Get(signals.Domain)

	return keepZscore(categories, contentVec, prior, alpha), nil
}

// keepZscore keeps categories based on those that measure how many standard deviations above this document's mean
// category score a given category sits. This self-normalizes per document, which handles the case where one piece of
// content is just generically more "embeddable" (scores high against everything) than another. alpha controls how much
// weight the content embedding gets vs. the domain prior (0.7-0.85 keeps content dominant while letting the prior act
// as a stabilizer/tie-breaker, especially valuable for short or ambiguous content).
func keepZscore(
	categories []CategoryEmbedding,
	contentVec []float32,
	prior map[string]float32,
	alpha float32,
) []Category {
	scores := make([]float32, len(categories))
	var sum float32
	for i, c := range categories {
		scores[i] = cosineSimilarity(contentVec, c.Embedding)
		sum += scores[i]
	}
	mean := sum / float32(len(scores))

	var variance float32
	for _, s := range scores {
		variance += (s - mean) * (s - mean)
	}
	stddev := float32(math.Sqrt(float64(variance / float32(len(scores)))))

	var kept []Category
	for i, c := range categories {
		if z := (scores[i] - mean) / stddev; z >= 1.5 {
			priorScore := prior[c.Label]
			kept = append(kept, Category{Label: c.Label, Score: alpha*scores[i] + (1-alpha)*priorScore})

		}
	}
	// sort.Slice(kept, func(i, j int) bool { return kept[i].Score > kept[j].Score })

	slices.SortFunc(kept, func(a Category, b Category) int {
		return cmp.Compare(a.Label, b.Label)
	})
	return slices.CompactFunc(kept, func(a Category, b Category) bool {
		return a.Label == b.Label
	})
}

// keepRelative keeps categories within some distance of the best match.
func keepRelative(categories []CategoryEmbedding, contentVec []float32) []Category {
	scored := make([]Category, len(categories))
	best := float32(-1)
	for i, c := range categories {
		score := cosineSimilarity(contentVec, c.Embedding)
		scored[i] = Category{Label: c.Label, Score: score}
		if score > best {
			best = score
		}
	}

	slices.SortFunc(scored, func(a Category, b Category) int {
		return cmp.Compare(a.Score, b.Score)
	})
	slices.Reverse(scored)

	var kept []Category
	for category := range slices.Values(scored) {
		// Only keep categories that score within 0.2 of best scoring category.
		if best-category.Score > 0.2 {
			break // sorted descending, so nothing further qualifies either
		}
		// Only keep at most 5 categories total.
		kept = append(kept, category)
		if len(kept) >= 5 {
			break
		}
	}
	slices.SortFunc(kept, func(a Category, b Category) int {
		return cmp.Compare(a.Label, b.Label)
	})
	return slices.CompactFunc(kept, func(a Category, b Category) bool {
		return a.Label == b.Label
	})
}
