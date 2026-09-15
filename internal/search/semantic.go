package search

import (
	"context"
	"fmt"

	"github.com/pardnchiu/kuradb/internal/database"
	databaseHandler "github.com/pardnchiu/kuradb/internal/database/handler"
	"github.com/pardnchiu/kuradb/internal/openai"
	"github.com/pardnchiu/kuradb/internal/vector"
)

const (
	minScore = 0.3
)

func getSemantic(ctx context.Context, dbs map[string]*database.DB, name string, embedder openai.Embedder, qCache *openai.Cache, q string, limit int) ([]databaseHandler.FileRow, error) {
	db := dbs[name]
	v, ok := qCache.Get(q)
	if !ok {
		vecs, err := embedder.EmbedBatch(ctx, []string{q})
		if err != nil {
			return nil, fmt.Errorf("embedder.EmbedBatch: %w", err)
		}
		if len(vecs) != 1 {
			return nil, fmt.Errorf("unexpected response length")
		}
		v = vecs[0]
		qCache.Set(q, v)
	}

	hits, err := vector.Search(name, v, limit)
	if err != nil {
		return nil, fmt.Errorf("vector.Search: %w", err)
	}
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
