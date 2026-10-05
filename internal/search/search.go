package search

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/pardnchiu/kuradb/internal/database"
	databaseHandler "github.com/pardnchiu/kuradb/internal/database/handler"
	"github.com/pardnchiu/kuradb/internal/segmenter"
)

const (
	TargetKeyword = "keyword"
)

var ErrInvalidArgument = errors.New("invalid argument")

func Search(ctx context.Context, dbs map[string]*database.DB, name, q, target string, limit int) (map[string][]Group, error) {
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
	if target != "" && target != TargetKeyword {
		return nil, fmt.Errorf("%w: unknown target %q", ErrInvalidArgument, target)
	}

	keywords, err := segmenter.Tokenize(q)
	if err != nil {
		return nil, err
	}

	var rows []databaseHandler.FileRow
	if len(keywords) > 0 {
		rows, err = databaseHandler.SearchKeyword(db, ctx, keywords, limit)
		if err != nil {
			return nil, err
		}
	}

	return map[string][]Group{
		TargetKeyword: group(rows),
	}, nil
}
