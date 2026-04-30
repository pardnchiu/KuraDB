package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/pardnchiu/KuraDB/internal/database"
	databaseHandler "github.com/pardnchiu/KuraDB/internal/database/handler"
	"github.com/pardnchiu/KuraDB/internal/openai"
	"github.com/pardnchiu/KuraDB/internal/vector"
)

func loadQueryCache(ctx context.Context, db *database.DB, qcache *openai.Cache) {
	expectedBytes := openai.Dim() * 4
	loaded, skipped := 0, 0
	err := databaseHandler.LoadQueryCache(db, ctx, func(q string, blob []byte) error {
		if len(blob) != expectedBytes {
			skipped++
			return nil
		}
		v, derr := openai.Decode(blob)
		if derr != nil {
			skipped++
			slog.Warn("query_cache: decode",
				slog.String("query", q),
				slog.String("error", derr.Error()))
			return nil
		}
		qcache.Preload(q, v)
		loaded++
		return nil
	})
	if err != nil {
		slog.Warn("query_cache: load",
			slog.String("error", err.Error()))
		return
	}
	slog.Info("query_cache: loaded",
		slog.Int("loaded", loaded),
		slog.Int("skipped", skipped))
}

func loadCache(ctx context.Context, db *database.DB, cache *vector.Cache) error {
	count := 0
	err := databaseHandler.LoadEmbeding(db, ctx, func(id int64, blob []byte) error {
		v, derr := openai.Decode(blob)
		if derr != nil {
			slog.Warn("openai.Decode",
				slog.String("error", derr.Error()))
			return nil
		}
		cache.Set(id, v)
		count++
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func runEmbedder(
	ctx context.Context,
	db *database.DB,
	embedder openai.Embedder,
	cache *vector.Cache,
	interval time.Duration,
	batch int,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			embedTick(ctx, db, embedder, cache, batch)
		}
	}
}

func embedTick(ctx context.Context, db *database.DB, embedder openai.Embedder, cache *vector.Cache, batch int) {
	pending, err := databaseHandler.ListPending(db, ctx, batch)
	if err != nil {
		slog.Warn("embed: ListPending",
			slog.String("error", err.Error()))
		return
	}
	if len(pending) == 0 {
		return
	}

	texts := make([]string, len(pending))
	for i, p := range pending {
		texts[i] = p.Content
	}

	vectors, err := embedder.EmbedBatch(ctx, texts)
	if err != nil {
		slog.Warn("embed: EmbedBatch",
			slog.Int("batch", len(pending)),
			slog.String("error", err.Error()))
		return
	}
	if len(vectors) != len(pending) {
		slog.Warn("embed: vector count mismatch",
			slog.Int("want", len(pending)),
			slog.Int("got", len(vectors)))
		return
	}

	updates := make([]databaseHandler.EmbeddingItem, len(pending))
	for i, p := range pending {
		updates[i] = databaseHandler.EmbeddingItem{
			ID:        p.ID,
			Content:   p.Content,
			Embedding: openai.Encode(vectors[i]),
		}
	}

	applied, err := databaseHandler.UpdateEmbedding(db, ctx, updates)
	if err != nil {
		slog.Warn("embed: SetEmbeddings",
			slog.String("error", err.Error()))
		return
	}

	if cache != nil && len(applied) > 0 {
		appliedSet := make(map[int64]struct{}, len(applied))
		for _, id := range applied {
			appliedSet[id] = struct{}{}
		}
		for i, p := range pending {
			if _, ok := appliedSet[p.ID]; ok {
				cache.Set(p.ID, vectors[i])
			}
		}
	}

	slog.Info("embedded",
		slog.Int("batch", len(pending)),
		slog.Int("applied", len(applied)))
}
