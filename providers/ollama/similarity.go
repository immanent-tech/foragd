/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package ollama

import "math"

func cosineSimilarity(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	sqrtA := mathSqrt(normA)
	sqrtB := mathSqrt(normB)
	if sqrtA < 0 || sqrtB < 0 {
		panic("cosineSimilarity: invalid norm") // or return error
	}
	return dot / (sqrtA * sqrtB)
}

func mathSqrt(f float64) float64 {
	return math.Sqrt(float64(f))
}
