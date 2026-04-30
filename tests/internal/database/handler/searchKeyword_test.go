package databaseHandler_integration_test

import (
	"context"
	"strings"
	"testing"

	databaseHandler "github.com/pardnchiu/KuraDB/internal/database/handler"
)

func TestSearchKeyword_Nominal(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 3, []string{
		"the quick brown fox",
		"lazy dog sleeps",
		"quick fox jumps over the lazy dog",
	})

	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"quick", "fox"}, 10)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Rank < rows[1].Rank {
		t.Errorf("rank not descending: %v < %v", rows[0].Rank, rows[1].Rank)
	}
	if !strings.Contains(rows[0].Content, "quick") || !strings.Contains(rows[0].Content, "fox") {
		t.Errorf("top row should contain both keywords, got %q", rows[0].Content)
	}
}

func TestSearchKeyword_OrSemantics(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 2, []string{"only apple here", "only banana here"})

	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"apple", "banana"}, 10)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("len(rows) = %d, want 2 (OR mode should match either)", len(rows))
	}
}

func TestSearchKeyword_CaseInsensitive(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 1, []string{"HELLO World"})

	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"hello"}, 10)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("case-insensitive match failed: got %d rows", len(rows))
	}
}

func TestSearchKeyword_Limit(t *testing.T) {
	db := openPerDB(t)
	contents := make([]string, 5)
	for i := range contents {
		contents[i] = "match content"
	}
	seedFileData(t, db, "/a", 5, contents)

	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"match"}, 2)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("len(rows) = %d, want 2 (limit)", len(rows))
	}
}

func TestSearchKeyword_DefaultLimitOnZero(t *testing.T) {
	db := openPerDB(t)
	contents := make([]string, 15)
	for i := range contents {
		contents[i] = "match"
	}
	seedFileData(t, db, "/a", 15, contents)

	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"match"}, 0)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 10 {
		t.Errorf("len(rows) = %d, want 10 (default limit applied for limit<=0)", len(rows))
	}
}

func TestSearchKeyword_DefaultLimitOnNegative(t *testing.T) {
	db := openPerDB(t)
	contents := make([]string, 12)
	for i := range contents {
		contents[i] = "match"
	}
	seedFileData(t, db, "/a", 12, contents)

	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"match"}, -5)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 10 {
		t.Errorf("len(rows) = %d, want 10 (default limit for negative)", len(rows))
	}
}

func TestSearchKeyword_FiltersDismissed(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 1, []string{"banana split"})
	if err := databaseHandler.Dismiss(db, context.Background(), "/a"); err != nil {
		t.Fatalf("Dismiss: %v", err)
	}
	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"banana"}, 10)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0 (dismissed must be filtered)", len(rows))
	}
}

func TestSearchKeyword_NilDB(t *testing.T) {
	if _, err := databaseHandler.SearchKeyword(nil, context.Background(), []string{"x"}, 10); err == nil {
		t.Fatal("SearchKeyword(nil) returned no error")
	}
}

func TestSearchKeyword_EmptyKeywords(t *testing.T) {
	db := openPerDB(t)
	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{}, 10)
	if err != nil {
		t.Fatalf("SearchKeyword(empty): %v", err)
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil", rows)
	}
}

func TestSearchKeyword_NilKeywords(t *testing.T) {
	db := openPerDB(t)
	rows, err := databaseHandler.SearchKeyword(db, context.Background(), nil, 10)
	if err != nil {
		t.Fatalf("SearchKeyword(nil): %v", err)
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil", rows)
	}
}

func TestSearchKeyword_CanceledCtx(t *testing.T) {
	db := openPerDB(t)
	if _, err := databaseHandler.SearchKeyword(db, canceledCtx(), []string{"x"}, 10); err == nil {
		t.Fatal("SearchKeyword canceled ctx returned no error")
	}
}

func TestSearchKeyword_NoMatch(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 1, []string{"hello world"})
	rows, err := databaseHandler.SearchKeyword(db, context.Background(), []string{"absent"}, 10)
	if err != nil {
		t.Fatalf("SearchKeyword: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0", len(rows))
	}
}
