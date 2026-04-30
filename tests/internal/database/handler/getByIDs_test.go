package databaseHandler_integration_test

import (
	"context"
	"testing"

	databaseHandler "github.com/pardnchiu/KuraDB/internal/database/handler"
)

func TestGetByIDs_Nominal(t *testing.T) {
	db := openPerDB(t)
	ids := seedFileData(t, db, "/a", 3, []string{"alpha", "beta", "gamma"})

	rows, err := databaseHandler.GetByIDs(db, context.Background(), ids)
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}

	got := map[int64]string{}
	for _, r := range rows {
		got[r.ID] = r.Content
		if r.Source != "/a" {
			t.Errorf("row.Source = %q, want /a", r.Source)
		}
		if r.Total != 3 {
			t.Errorf("row.Total = %d, want 3", r.Total)
		}
	}
	want := map[int64]string{ids[0]: "alpha", ids[1]: "beta", ids[2]: "gamma"}
	for id, content := range want {
		if got[id] != content {
			t.Errorf("id=%d content=%q, want %q", id, got[id], content)
		}
	}
}

func TestGetByIDs_NilDB(t *testing.T) {
	if _, err := databaseHandler.GetByIDs(nil, context.Background(), []int64{1}); err == nil {
		t.Fatal("GetByIDs(nil) returned no error")
	}
}

func TestGetByIDs_EmptyIDs(t *testing.T) {
	db := openPerDB(t)
	rows, err := databaseHandler.GetByIDs(db, context.Background(), []int64{})
	if err != nil {
		t.Fatalf("GetByIDs(empty): %v", err)
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil", rows)
	}
}

func TestGetByIDs_NilIDs(t *testing.T) {
	db := openPerDB(t)
	rows, err := databaseHandler.GetByIDs(db, context.Background(), nil)
	if err != nil {
		t.Fatalf("GetByIDs(nil ids): %v", err)
	}
	if rows != nil {
		t.Errorf("rows = %v, want nil", rows)
	}
}

func TestGetByIDs_CanceledCtx(t *testing.T) {
	db := openPerDB(t)
	if _, err := databaseHandler.GetByIDs(db, canceledCtx(), []int64{1}); err == nil {
		t.Fatal("GetByIDs with canceled ctx returned no error")
	}
}

func TestGetByIDs_FiltersDismissed(t *testing.T) {
	db := openPerDB(t)
	ids := seedFileData(t, db, "/a", 2, []string{"x", "y"})
	if err := databaseHandler.Dismiss(db, context.Background(), "/a"); err != nil {
		t.Fatalf("Dismiss: %v", err)
	}
	rows, err := databaseHandler.GetByIDs(db, context.Background(), ids)
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0 (dismissed rows must be filtered)", len(rows))
	}
}

func TestGetByIDs_UnknownID(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 1, []string{"only"})

	rows, err := databaseHandler.GetByIDs(db, context.Background(), []int64{999_999})
	if err != nil {
		t.Fatalf("GetByIDs: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("len(rows) = %d, want 0", len(rows))
	}
}
