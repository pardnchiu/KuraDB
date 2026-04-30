package parser_integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pardnchiu/KuraDB/internal/filesystem/parser"
)

func TestMarkdown(t *testing.T) {
	dir := t.TempDir()

	happyPath := filepath.Join(dir, "happy.md")
	if err := os.WriteFile(happyPath, []byte("# Heading\n\nFirst paragraph.\n\nSecond paragraph.\n"), 0o644); err != nil {
		t.Fatalf("write happy: %v", err)
	}

	emptyPath := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	whitespacePath := filepath.Join(dir, "ws.md")
	if err := os.WriteFile(whitespacePath, []byte("   \n\n   \n"), 0o644); err != nil {
		t.Fatalf("write whitespace: %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		path    string
		wantErr bool
		wantLen int
	}{
		{"nominal multi-paragraph", context.Background(), happyPath, false, 3},
		{"empty path", context.Background(), "", true, 0},
		{"non-existent path", context.Background(), filepath.Join(dir, "nope.md"), true, 0},
		{"empty file", context.Background(), emptyPath, true, 0},
		{"whitespace-only file", context.Background(), whitespacePath, true, 0},
		{"cancelled context", cancelled, happyPath, true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Markdown(tt.ctx, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Markdown() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("Markdown() got %d docs, want %d", len(got), tt.wantLen)
			}
			for i, d := range got {
				if d.Source != tt.path {
					t.Errorf("chunk[%d].Source = %q, want %q", i, d.Source, tt.path)
				}
				if d.Index != i+1 {
					t.Errorf("chunk[%d].Index = %d, want %d", i, d.Index, i+1)
				}
				if d.Total != len(got) {
					t.Errorf("chunk[%d].Total = %d, want %d", i, d.Total, len(got))
				}
				if d.Content == "" {
					t.Errorf("chunk[%d].Content empty", i)
				}
			}
		})
	}
}
