package filesystem_integration_test

import (
	"context"
	"testing"

	"github.com/pardnchiu/KuraDB/internal/filesystem"
)

// WalkFiles requires *database.DB (cgo SQLite) for real traversal, so only
// the cancelled-context fast path is exercised here. Per the integration
// skill rules, cgo / external-service dependencies belong elsewhere.
func TestWalkFiles_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := filesystem.WalkFiles(ctx, ".", ".", nil, nil); got != nil {
		t.Errorf("WalkFiles with cancelled context returned non-nil result")
	}
}
