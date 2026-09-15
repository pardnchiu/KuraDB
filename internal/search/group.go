package search

import (
	databaseHandler "github.com/pardnchiu/kuradb/internal/database/handler"
)

const (
	DefaultLimit = 10
	MaxLimit     = 100
)

type Match struct {
	Chunk   int    `json:"chunk"`
	Content string `json:"content"`
}

type Group struct {
	Source  string  `json:"source"`
	Matches []Match `json:"matches"`
}

func group(flat []databaseHandler.FileRow) []Group {
	if len(flat) == 0 {
		return []Group{}
	}
	idx := make(map[string]int, len(flat))
	groups := make([]Group, 0)
	for _, h := range flat {
		i, ok := idx[h.Source]
		if !ok {
			idx[h.Source] = len(groups)
			groups = append(groups, Group{Source: h.Source})
			i = idx[h.Source]
		}
		groups[i].Matches = append(groups[i].Matches, Match{
			Chunk:   h.Chunk,
			Content: h.Content,
		})
	}
	return groups
}
