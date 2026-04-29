package parser

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var pptxSlidePattern = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

func PPTX(ctx context.Context, path string) ([]FileData, error) {
	if path == "" {
		return nil, fmt.Errorf("pptx: path is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("pptx: zip.OpenReader: %w", err)
	}
	defer zr.Close()

	type slideEntry struct {
		num int
		zf  *zip.File
	}
	var slides []slideEntry
	for _, f := range zr.File {
		m := pptxSlidePattern.FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}
		n, perr := strconv.Atoi(m[1])
		if perr != nil {
			continue
		}
		slides = append(slides, slideEntry{num: n, zf: f})
	}
	if len(slides) == 0 {
		return nil, fmt.Errorf("pptx: %q no slides", path)
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].num < slides[j].num })

	total := len(slides)
	docs := make([]FileData, 0, total)
	for i, s := range slides {
		if err := ctx.Err(); err != nil {
			return docs, err
		}
		rc, oerr := s.zf.Open()
		if oerr != nil {
			return docs, fmt.Errorf("pptx: slide %d open: %w", s.num, oerr)
		}
		text, terr := pptxSlideText(ctx, rc)
		rc.Close()
		if terr != nil {
			return docs, fmt.Errorf("pptx: slide %d parse: %w", s.num, terr)
		}
		docs = append(docs, FileData{
			Source:  path,
			Index:   i + 1,
			Total:   total,
			Content: text,
		})
	}
	return docs, nil
}

func pptxSlideText(ctx context.Context, r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var b strings.Builder
	for {
		if err := ctx.Err(); err != nil {
			return b.String(), err
		}
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return b.String(), err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				var content string
				if err := dec.DecodeElement(&content, &t); err != nil {
					return b.String(), err
				}
				b.WriteString(content)
			case "br":
				b.WriteByte('\n')
			}
		case xml.EndElement:
			if t.Name.Local == "p" {
				b.WriteByte('\n')
			}
		}
	}
	return b.String(), nil
}
