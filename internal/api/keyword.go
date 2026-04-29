package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
	databaseHandler "github.com/pardnchiu/AgenvoyRAG/internal/database/handler"
	"github.com/pardnchiu/AgenvoyRAG/internal/segmenter"
)

const (
	defaultLimit = 10
	maxLimit     = 100
)

type Match struct {
	Chunk   int    `json:"chunk"`
	Content string `json:"content"`
}

type Group struct {
	Source  string  `json:"source"`
	Matches []Match `json:"matches"`
	rank    float64
}

func Keyword(db *database.DB, seg *segmenter.Segmenter) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
			return
		}
		limit := parseLimit(c)

		flat, err := getKeyword(c.Request.Context(), db, seg, q, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"results": groupKeyword(flat)})
	}
}

func getKeyword(ctx context.Context, db *database.DB, seg *segmenter.Segmenter, q string, limit int) ([]databaseHandler.FileRow, error) {
	keywords := seg.Tokenize(q)
	if len(keywords) == 0 {
		return nil, nil
	}

	rows, err := databaseHandler.SearchKeyword(db, ctx, keywords, limit)
	if err != nil {
		return nil, fmt.Errorf("databaseHandler.SearchKeyword: %w", err)
	}

	results := make([]databaseHandler.FileRow, 0, len(rows))
	for _, r := range rows {
		results = append(results, r)
	}
	return results, nil
}

func groupKeyword(flat []databaseHandler.FileRow) []Group {
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
			})
			i = idx[h.Source]
		}
		groups[i].rank += float64(h.Rank)
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

func parseLimit(c *gin.Context) int {
	raw := c.Query("limit")
	if raw == "" {
		return defaultLimit
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 || v > maxLimit {
		return defaultLimit
	}
	return v
}
