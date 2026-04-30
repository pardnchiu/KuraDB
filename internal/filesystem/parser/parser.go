package parser

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxParagraphRunes = 65535

var (
	paragraphRe       = regexp.MustCompile(`\n[\t ]*\n+`)
	sentenceTerminals = map[rune]struct{}{
		'.': {}, '?': {}, '!': {},
		'。': {}, '？': {}, '！': {},
	}
)

func splitParagraphs(ctx context.Context, source, text string) []FileData {
	parts := paragraphRe.Split(text, -1)
	chunks := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) <= maxParagraphRunes {
			chunks = append(chunks, t)
			continue
		}
		chunks = append(chunks, splitBySentence(t, maxParagraphRunes)...)
	}
	if len(chunks) == 0 {
		return nil
	}

	total := len(chunks)
	docs := make([]FileData, 0, total)
	for i, p := range chunks {
		if err := ctx.Err(); err != nil {
			return docs
		}
		docs = append(docs, FileData{
			Source:  source,
			Index:   i + 1,
			Total:   total,
			Content: p,
		})
	}
	return docs
}

func splitBySentence(text string, max int) []string {
	runes := []rune(text)
	if len(runes) <= max {
		if seg := strings.TrimSpace(string(runes)); seg != "" {
			return []string{seg}
		}
		return nil
	}

	var boundaries []int
	for i, r := range runes {
		if _, ok := sentenceTerminals[r]; ok {
			boundaries = append(boundaries, i)
		}
	}

	var chunks []string
	start := 0
	bIdx := 0
	for start < len(runes) {
		if len(runes)-start <= max {
			if seg := strings.TrimSpace(string(runes[start:])); seg != "" {
				chunks = append(chunks, seg)
			}
			break
		}
		limit := start + max - 1
		for bIdx < len(boundaries) && boundaries[bIdx] < start {
			bIdx++
		}
		cut := -1
		for bIdx < len(boundaries) && boundaries[bIdx] <= limit {
			cut = boundaries[bIdx]
			bIdx++
		}
		if cut < start {
			cut = limit
		}
		if seg := strings.TrimSpace(string(runes[start : cut+1])); seg != "" {
			chunks = append(chunks, seg)
		}
		start = cut + 1
	}
	return chunks
}
