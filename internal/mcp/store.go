package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/pardnchiu/kuradb/internal/database"
	"github.com/pardnchiu/kuradb/internal/openai"
	"github.com/pardnchiu/kuradb/internal/search"
)

type store struct {
	reg      *database.Registry
	dbs      map[string]*database.DB
	embedder openai.Embedder
	qCache   *openai.Cache
}

func newStore(reg *database.Registry, dbs map[string]*database.DB, embedder openai.Embedder, qCache *openai.Cache) *store {
	return &store{reg: reg, dbs: dbs, embedder: embedder, qCache: qCache}
}

func (s *store) list(_ context.Context) (listOutput, error) {
	entries, err := s.reg.Load()
	if err != nil {
		return listOutput{}, fmt.Errorf("registry.Load: %w", err)
	}
	if entries == nil {
		entries = []database.Entry{}
	}

	loaded := make([]string, 0, len(s.dbs))
	for name := range s.dbs {
		loaded = append(loaded, name)
	}

	return listOutput{Loaded: loaded, Registered: entries}, nil
}

func (s *store) search(ctx context.Context, in searchInput) (searchOutput, error) {
	mode := strings.ToLower(strings.TrimSpace(in.Mode))

	dic, err := search.Search(ctx, s.dbs, s.embedder, s.qCache,
		strings.TrimSpace(in.DB), strings.TrimSpace(in.Q), mode, in.Limit)
	if err != nil {
		return searchOutput{}, err
	}

	return searchOutput{
		Keyword:  dic[search.TargetKeyword],
		Semantic: dic[search.TargetSemantic],
	}, nil
}
