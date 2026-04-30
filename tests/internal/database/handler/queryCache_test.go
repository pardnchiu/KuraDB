package databaseHandler_integration_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	databaseHandler "github.com/pardnchiu/KuraDB/internal/database/handler"
)

func TestSaveQueryCache_Nominal(t *testing.T) {
	db := openGlobalDB(t)
	blob := []byte{0x01, 0x02, 0x03, 0x04}

	if err := databaseHandler.SaveQueryCache(db, context.Background(), "hello", blob); err != nil {
		t.Fatalf("SaveQueryCache: %v", err)
	}

	var got []byte
	row := db.DB.QueryRowContext(context.Background(),
		`SELECT embedding FROM query_cache WHERE query = ?`, "hello")
	if err := row.Scan(&got); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !bytes.Equal(got, blob) {
		t.Errorf("blob = %v, want %v", got, blob)
	}
}

func TestSaveQueryCache_Upsert(t *testing.T) {
	db := openGlobalDB(t)
	first := []byte{0x01, 0x02}
	second := []byte{0xAA, 0xBB, 0xCC}

	if err := databaseHandler.SaveQueryCache(db, context.Background(), "k", first); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := databaseHandler.SaveQueryCache(db, context.Background(), "k", second); err != nil {
		t.Fatalf("second save: %v", err)
	}

	var got []byte
	row := db.DB.QueryRowContext(context.Background(),
		`SELECT embedding FROM query_cache WHERE query = ?`, "k")
	if err := row.Scan(&got); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !bytes.Equal(got, second) {
		t.Errorf("blob = %v, want overwritten %v", got, second)
	}
}

func TestSaveQueryCache_NilDB(t *testing.T) {
	if err := databaseHandler.SaveQueryCache(nil, context.Background(), "q", []byte{1}); err == nil {
		t.Fatal("SaveQueryCache(nil) returned no error")
	}
}

func TestSaveQueryCache_EmptyQuery(t *testing.T) {
	db := openGlobalDB(t)
	if err := databaseHandler.SaveQueryCache(db, context.Background(), "", []byte{1}); err == nil {
		t.Fatal("SaveQueryCache empty query returned no error")
	}
}

func TestSaveQueryCache_NilBlob(t *testing.T) {
	db := openGlobalDB(t)
	if err := databaseHandler.SaveQueryCache(db, context.Background(), "q", nil); err == nil {
		t.Fatal("SaveQueryCache nil blob returned no error")
	}
}

func TestSaveQueryCache_EmptyBlob(t *testing.T) {
	db := openGlobalDB(t)
	if err := databaseHandler.SaveQueryCache(db, context.Background(), "q", []byte{}); err == nil {
		t.Fatal("SaveQueryCache empty blob returned no error")
	}
}

func TestSaveQueryCache_CanceledCtx(t *testing.T) {
	db := openGlobalDB(t)
	if err := databaseHandler.SaveQueryCache(db, canceledCtx(), "q", []byte{1}); err == nil {
		t.Fatal("SaveQueryCache canceled ctx returned no error")
	}
}

func TestLoadQueryCache_Nominal(t *testing.T) {
	db := openGlobalDB(t)
	want := map[string][]byte{
		"a":      {1, 2},
		"second": {0xff, 0x00, 0xab},
	}
	for q, blob := range want {
		if err := databaseHandler.SaveQueryCache(db, context.Background(), q, blob); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got := map[string][]byte{}
	err := databaseHandler.LoadQueryCache(db, context.Background(), func(q string, blob []byte) error {
		got[q] = append([]byte(nil), blob...)
		return nil
	})
	if err != nil {
		t.Fatalf("LoadQueryCache: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for q, blob := range want {
		if !bytes.Equal(got[q], blob) {
			t.Errorf("query %q blob = %v, want %v", q, got[q], blob)
		}
	}
}

func TestLoadQueryCache_Empty(t *testing.T) {
	db := openGlobalDB(t)
	calls := 0
	err := databaseHandler.LoadQueryCache(db, context.Background(), func(q string, blob []byte) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("LoadQueryCache(empty): %v", err)
	}
	if calls != 0 {
		t.Errorf("fn called %d times, want 0", calls)
	}
}

func TestLoadQueryCache_NilDB(t *testing.T) {
	err := databaseHandler.LoadQueryCache(nil, context.Background(), func(q string, blob []byte) error { return nil })
	if err == nil {
		t.Fatal("LoadQueryCache(nil db) returned no error")
	}
}

func TestLoadQueryCache_NilFn(t *testing.T) {
	db := openGlobalDB(t)
	if err := databaseHandler.LoadQueryCache(db, context.Background(), nil); err == nil {
		t.Fatal("LoadQueryCache(nil fn) returned no error")
	}
}

func TestLoadQueryCache_CanceledCtx(t *testing.T) {
	db := openGlobalDB(t)
	if err := databaseHandler.SaveQueryCache(db, context.Background(), "q", []byte{1}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	err := databaseHandler.LoadQueryCache(db, canceledCtx(), func(q string, blob []byte) error { return nil })
	if err == nil {
		t.Fatal("LoadQueryCache canceled ctx returned no error")
	}
}

func TestLoadQueryCache_FnError(t *testing.T) {
	db := openGlobalDB(t)
	if err := databaseHandler.SaveQueryCache(db, context.Background(), "q", []byte{1}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sentinel := errors.New("boom")
	err := databaseHandler.LoadQueryCache(db, context.Background(), func(q string, blob []byte) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel %v", err, sentinel)
	}
}
