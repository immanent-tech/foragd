// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package ollama

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// URLSignals extracts classification-relevant features from a content URL.
type URLSignals struct {
	Domain       string   // "example.com", "www." stripped
	PathSegments []string // human-readable path tokens; numeric IDs/UUIDs dropped
}

func ExtractURLSignals(rawURL string) (URLSignals, error) {
	urlSignal, err := url.Parse(rawURL)
	if err != nil {
		return URLSignals{}, fmt.Errorf("parsing URL: %w", err)
	}

	domain := strings.ToLower(urlSignal.Hostname())
	domain = strings.TrimPrefix(domain, "www.")

	var segments []string
	for part := range strings.SplitSeq(strings.Trim(urlSignal.Path, "/"), "/") {
		if part == "" || isNumericOrID(part) {
			continue // drop article IDs, UUIDs, pure numbers -- noise, not taxonomy
		}
		words := strings.FieldsFunc(part, func(r rune) bool { return r == '-' || r == '_' })
		segments = append(segments, strings.Join(words, " "))
	}
	return URLSignals{Domain: domain, PathSegments: segments}, nil
}

func isNumericOrID(segment string) bool {
	if len(segment) >= 8 && strings.Count(segment, "-") >= 4 {
		return true // looks like a UUID
	}
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(segment) > 0
}

// DomainCategoryPrior tracks a learned or curated category distribution per domain. Safe for concurrent use.
type DomainCategoryPrior struct {
	mu     sync.RWMutex
	priors map[string]map[string]float32 // domain -> label -> weight (normalized to sum 1)
}

func NewDomainCategoryPrior() *DomainCategoryPrior {
	return &DomainCategoryPrior{priors: make(map[string]map[string]float32)}
}

// Seed manually associates a domain with known category weights, e.g. {"Technology & Computing": 0.9, "Business and
// Finance": 0.1} for a publisher you already know the beat of.
func (p *DomainCategoryPrior) Seed(domain string, weights map[string]float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.priors[domain] = normalizeWeights(weights)
}

// Observe updates a domain's prior via exponential moving average each time a document from it gets classified -- so
// priors self-improve from your own pipeline's output without manual curation. Use a small alpha (0.05-0.15) so one
// mis-classified outlier can't swing a domain's prior.
func (p *DomainCategoryPrior) Observe(domain, label string, alpha float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	dist, ok := p.priors[domain]
	if !ok {
		dist = make(map[string]float32)
	}
	for k := range dist {
		dist[k] *= (1 - alpha)
	}
	dist[label] += alpha
	p.priors[domain] = normalizeWeights(dist)
}

func (p *DomainCategoryPrior) Get(domain string) map[string]float32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.priors[domain] // nil for unseen domains -- treated as zero everywhere, harmless
}

func normalizeWeights(w map[string]float32) map[string]float32 {
	var sum float32
	for _, v := range w {
		sum += v
	}
	if sum == 0 {
		return w
	}
	out := make(map[string]float32, len(w))
	for k, v := range w {
		out[k] = v / sum
	}
	return out
}

func BuildPriors() *DomainCategoryPrior {
	priors := NewDomainCategoryPrior()
	priors.Seed("techcrunch.com", map[string]float32{"Technology & Computing": 1.0})
	priors.Seed("github.com", map[string]float32{"Technology & Computing": 1.0})
	return priors
}
