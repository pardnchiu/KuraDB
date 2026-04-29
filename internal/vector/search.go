package vector

import (
	"math"
	"sort"
)

type Hit struct {
	ID    int64
	Score float64
}

func (c *Cache) Search(query []float32, topK int) []Hit {
	if c == nil || topK <= 0 || len(query) == 0 {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.vecMap) == 0 {
		return nil
	}

	hits := make([]Hit, 0, len(c.vecMap))
	for id, v := range c.vecMap {
		if len(v) != len(query) {
			continue
		}
		hits = append(hits, Hit{ID: id, Score: cosine(query, v)})
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})

	if topK > len(hits) {
		topK = len(hits)
	}
	return hits[:topK]
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i, x := range a {
		y := b[i]
		fx, fy := float64(x), float64(y)
		dot += fx * fy
		na += fx * fx
		nb += fy * fy
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
