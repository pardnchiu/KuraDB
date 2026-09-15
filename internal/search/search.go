package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/pardnchiu/kuradb/internal/database"
	databaseHandler "github.com/pardnchiu/kuradb/internal/database/handler"
	"github.com/pardnchiu/kuradb/internal/openai"
	"github.com/pardnchiu/kuradb/internal/segmenter"
)

const (
	TargetKeyword  = "keyword"
	TargetSemantic = "semantic"
)

var ErrInvalidArgument = errors.New("invalid argument")

func Search(ctx context.Context, dbs map[string]*database.DB, embedder openai.Embedder, qCache *openai.Cache, name, q, target string, limit int) (map[string][]Group, error) {
	db, ok := dbs[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q not exist", ErrInvalidArgument, name)
	}
	if q == "" {
		return nil, fmt.Errorf("%w: q is required", ErrInvalidArgument)
	}
	if limit <= 0 || limit > MaxLimit {
		limit = DefaultLimit
	}

	target = strings.ToLower(target)
	runKeyword := target == "" || target == TargetKeyword
	runSemantic := target == "" || target == TargetSemantic
	if !runKeyword && !runSemantic {
		return nil, fmt.Errorf("%w: unknown target %q", ErrInvalidArgument, target)
	}

	var (
		keywordResults  []databaseHandler.FileRow
		semanticResults []databaseHandler.FileRow
		kwErr           error
		semErr          error
		wg              sync.WaitGroup
	)

	if runKeyword {
		wg.Go(func() {
			keywords, err := segmenter.Tokenize(q)
			if err != nil {
				kwErr = err
				return
			}
			if len(keywords) == 0 {
				return
			}
			keywordResults, kwErr = databaseHandler.SearchKeyword(db, ctx, keywords, limit)
		})
	}

	if runSemantic {
		wg.Go(func() {
			semanticResults, semErr = getSemantic(ctx, dbs, name, embedder, qCache, q, limit)
		})
	}

	wg.Wait()

	if kwErr != nil {
		return nil, kwErr
	}
	if semErr != nil {
		return nil, semErr
	}

	dic := make(map[string][]Group, 2)
	if runKeyword {
		dic[TargetKeyword] = group(keywordResults)
	}
	if runSemantic {
		dic[TargetSemantic] = group(semanticResults)
	}
	return dic, nil
}
