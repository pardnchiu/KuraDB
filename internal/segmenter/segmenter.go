package segmenter

import (
	"fmt"
	"strings"

	"github.com/go-ego/gse"
)

type Segmenter struct {
	seg gse.Segmenter
}

func New() (*Segmenter, error) {
	var seg gse.Segmenter
	if err := seg.LoadDictEmbed("zh_s"); err != nil {
		return nil, fmt.Errorf("seg.LoadDictEmbed: %w", err)
	}

	if err := seg.LoadStopEmbed(); err != nil {
		return nil, fmt.Errorf("seg.LoadStopEmbed: %w", err)
	}
	return &Segmenter{seg: seg}, nil
}

func (s *Segmenter) Tokenize(text string) []string {
	if s == nil {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	raw := s.seg.Cut(text, true)
	trimmed := s.seg.Trim(raw)

	seen := make(map[string]struct{}, len(trimmed))
	out := make([]string, 0, len(trimmed))
	for _, w := range trimmed {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	return out
}
