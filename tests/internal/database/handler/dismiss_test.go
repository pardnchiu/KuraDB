package databaseHandler_integration_test

import (
	"context"
	"testing"

	databaseHandler "github.com/pardnchiu/KuraDB/internal/database/handler"
)

func TestDismiss_Nominal(t *testing.T) {
	db := openPerDB(t)
	src := "/abs/path/file.txt"
	seedFileData(t, db, src, 2, []string{"alpha", "beta"})

	if err := databaseHandler.Dismiss(db, context.Background(), src); err != nil {
		t.Fatalf("Dismiss: %v", err)
	}

	var dismissed int
	row := db.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM file_data WHERE source = ? AND dismiss = TRUE`, src)
	if err := row.Scan(&dismissed); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if dismissed != 2 {
		t.Errorf("dismissed = %d, want 2", dismissed)
	}
}

func TestDismiss_NilDB(t *testing.T) {
	if err := databaseHandler.Dismiss(nil, context.Background(), "x"); err == nil {
		t.Fatal("Dismiss(nil) returned no error")
	}
}

func TestDismiss_EmptySource(t *testing.T) {
	db := openPerDB(t)
	if err := databaseHandler.Dismiss(db, context.Background(), ""); err == nil {
		t.Fatal("Dismiss with empty source returned no error")
	}
}

func TestDismiss_CanceledCtx(t *testing.T) {
	db := openPerDB(t)
	if err := databaseHandler.Dismiss(db, canceledCtx(), "x"); err == nil {
		t.Fatal("Dismiss with canceled ctx returned no error")
	}
}

func TestDismiss_NonexistentSource(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 1, []string{"x"})

	if err := databaseHandler.Dismiss(db, context.Background(), "/does-not-exist"); err != nil {
		t.Fatalf("Dismiss should be no-op for missing source, got: %v", err)
	}

	var dismissed int
	row := db.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM file_data WHERE dismiss = TRUE`)
	if err := row.Scan(&dismissed); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if dismissed != 0 {
		t.Errorf("dismissed = %d, want 0", dismissed)
	}
}

func TestDismiss_Idempotent(t *testing.T) {
	db := openPerDB(t)
	src := "/idem"
	seedFileData(t, db, src, 1, []string{"x"})

	if err := databaseHandler.Dismiss(db, context.Background(), src); err != nil {
		t.Fatalf("first Dismiss: %v", err)
	}
	if err := databaseHandler.Dismiss(db, context.Background(), src); err != nil {
		t.Fatalf("second Dismiss: %v", err)
	}
}
