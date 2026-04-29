package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
	databaseHandler "github.com/pardnchiu/AgenvoyRAG/internal/database/handler"
	"github.com/pardnchiu/AgenvoyRAG/internal/openai"
	"github.com/pardnchiu/AgenvoyRAG/internal/vector"
)

const (
	minScore = 0.3
)

func Semantic(db *database.DB, cache *vector.Cache, embedder openai.Embedder, qcache *openai.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
			return
		}
		limit := parseLimit(c)

		flat, err := getSemantic(c.Request.Context(), db, cache, embedder, qcache, q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"results": groupSemantic(flat)})
	}
}

func getSemantic(ctx context.Context, db *database.DB, cache *vector.Cache, embedder openai.Embedder, qCache *openai.Cache, q string, limit int) ([]databaseHandler.FileRow, error) {
	vector, ok := qCache.Get(q)
	if !ok {
		vecs, err := embedder.EmbedBatch(ctx, []string{q})
		if err != nil {
			return nil, fmt.Errorf("embedder.EmbedBatch: %w", err)
		}
		if len(vecs) != 1 {
			return nil, fmt.Errorf("unexpected response length")
		}
		vector = vecs[0]
		qCache.Set(q, vector)
	}

	hits := cache.Search(vector, limit)
	cutoff := len(hits)
	for i, h := range hits {
		if h.Score < minScore {
			cutoff = i
			break
		}
	}

	hits = hits[:cutoff]
	if len(hits) == 0 {
		return nil, nil
	}

	ids := make([]int64, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	rows, err := databaseHandler.GetByIDs(db, ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("getByIDs: %w", err)
	}

	rowMap := make(map[int64]databaseHandler.FileRow, len(rows))
	for _, r := range rows {
		rowMap[r.ID] = r
	}

	out := make([]databaseHandler.FileRow, 0, len(hits))
	for _, h := range hits {
		r, ok := rowMap[h.ID]
		if !ok {
			continue
		}
		out = append(out, databaseHandler.FileRow{
			ID:      h.ID,
			Source:  r.Source,
			Chunk:   r.Chunk,
			Content: r.Content,
			Rank:    h.Score,
		})
	}
	return out, nil
}

func groupSemantic(flat []databaseHandler.FileRow) []Group {
	if len(flat) == 0 {
		return []Group{}
	}

	idx := make(map[string]int, len(flat))
	groups := make([]Group, 0)
	for _, h := range flat {
		i, ok := idx[h.Source]
		if !ok {
			idx[h.Source] = len(groups)
			groups = append(groups, Group{
				Source: h.Source,
				rank:   h.Rank,
			})
			i = idx[h.Source]
		} else if h.Rank > groups[i].rank {
			groups[i].rank = h.Rank
		}
		groups[i].Matches = append(groups[i].Matches, Match{
			Chunk:   h.Chunk,
			Content: h.Content,
		})
	}

	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].rank > groups[j].rank
	})
	return groups
}
