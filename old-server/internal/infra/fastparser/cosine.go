package fastparser

import (
	"math"
)

// CosineSim calculates the cosine similarity between two vectors. Should we be using a graph-based approach? Does it make sense? I think most usecases are fairly few functions so a graph-based approach might be overkill. Think Hierarchical Navigable Small World (HNSW), there's a paper on this and maybe some GO libraries out there?
func CosineSim(a, b []float32) float64 {
	var dot, na, nb float64

	for i, v := range a {
		dot += float64(v * b[i])
		na += float64(v * v)
		nb += float64(b[i] * b[i])
	}

	// for i := 0; i < len(a); i++ {
	// 	dot += float64(a[i] * b[i])
	// 	na += float64(a[i] * a[i])
	// 	nb += float64(b[i] * b[i])
	// }

	if na == 0 || nb == 0 {
		return 0
	}

	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func CosineSimFloat64(a, b []float64) float64 {
	var dot, na2, nb2 float64
	for i := range a {
		dot += a[i] * b[i]
		na2 += a[i] * a[i]
		nb2 += b[i] * b[i]
	}
	if na2 == 0 || nb2 == 0 {
		return 0
	}
	return dot / (math.Sqrt(na2) * math.Sqrt(nb2))
}
