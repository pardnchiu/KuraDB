package search

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/pardnchiu/kuradb/internal/database"
	databaseHandler "github.com/pardnchiu/kuradb/internal/database/handler"
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

	var keywords []string
	for word := range strings.FieldsSeq(strings.ToLower(q)) {
		if !slices.Contains(keywords, word) {
			keywords = append(keywords, word)
		}
	}

	rows, err := databaseHandler.SearchKeyword(db, ctx, keywords, limit)
	if err != nil {
		return nil, err
	}

	return map[string][]Group{
		TargetKeyword: group(rows),
	}, nil
}
