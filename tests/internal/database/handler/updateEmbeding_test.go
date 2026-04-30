package databaseHandler_integration_test

import (
	"bytes"
	"context"
	"testing"

	databaseHandler "github.com/pardnchiu/KuraDB/internal/database/handler"
)

func TestUpdateEmbedding_Nominal(t *testing.T) {
	db := openPerDB(t)
	ids := seedFileData(t, db, "/a", 2, []string{"alpha", "beta"})

	emb0 := []byte{0x01, 0x02, 0x03, 0x04}
	emb1 := []byte{0x10, 0x20, 0x30, 0x40}
	items := []databaseHandler.EmbeddingItem{
		{ID: ids[0], Content: "alpha", Embedding: emb0},
		{ID: ids[1], Content: "beta", Embedding: emb1},
	}

	applied, err := databaseHandler.UpdateEmbedding(db, context.Background(), items)
	if err != nil {
		t.Fatalf("UpdateEmbedding: %v", err)
	}
	if len(applied) != 2 {
		t.Fatalf("applied = %v, want 2 IDs", applied)
	}

	for _, it := range items {
		var blob []byte
		var isEmbed bool
		row := db.DB.QueryRowContext(context.Background(),
			`SELECT embedding, is_embed FROM file_data WHERE id = ?`, it.ID)
		if err := row.Scan(&blob, &isEmbed); err != nil {
			t.Fatalf("scan id=%d: %v", it.ID, err)
		}
		if !bytes.Equal(blob, it.Embedding) {
			t.Errorf("id=%d blob = %v, want %v", it.ID, blob, it.Embedding)
		}
		if !isEmbed {
			t.Errorf("id=%d is_embed = false, want true", it.ID)
		}
	}
}

func TestUpdateEmbedding_NilDB(t *testing.T) {
	_, err := databaseHandler.UpdateEmbedding(nil, context.Background(),
		[]databaseHandler.EmbeddingItem{{ID: 1, Content: "x", Embedding: []byte{1}}})
	if err == nil {
		t.Fatal("UpdateEmbedding(nil) returned no error")
	}
}

func TestUpdateEmbedding_EmptyItems(t *testing.T) {
	db := openPerDB(t)
	applied, err := databaseHandler.UpdateEmbedding(db, context.Background(), nil)
	if err != nil {
		t.Fatalf("UpdateEmbedding(nil items): %v", err)
	}
	if applied != nil {
		t.Errorf("applied = %v, want nil", applied)
	}

	applied, err = databaseHandler.UpdateEmbedding(db, context.Background(), []databaseHandler.EmbeddingItem{})
	if err != nil {
		t.Fatalf("UpdateEmbedding(empty items): %v", err)
	}
	if applied != nil {
		t.Errorf("applied = %v, want nil", applied)
	}
}

func TestUpdateEmbedding_CanceledCtx(t *testing.T) {
	db := openPerDB(t)
	_, err := databaseHandler.UpdateEmbedding(db, canceledCtx(),
		[]databaseHandler.EmbeddingItem{{ID: 1, Content: "x", Embedding: []byte{1}}})
	if err == nil {
		t.Fatal("UpdateEmbedding canceled ctx returned no error")
	}
}

func TestUpdateEmbedding_ContentMismatchSkipped(t *testing.T) {
	db := openPerDB(t)
	ids := seedFileData(t, db, "/a", 1, []string{"alpha"})

	items := []databaseHandler.EmbeddingItem{
		{ID: ids[0], Content: "STALE_CONTENT", Embedding: []byte{0x99}},
	}
	applied, err := databaseHandler.UpdateEmbedding(db, context.Background(), items)
	if err != nil {
		t.Fatalf("UpdateEmbedding: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("applied = %v, want empty (stale content must skip)", applied)
	}

	var isEmbed bool
	row := db.DB.QueryRowContext(context.Background(),
		`SELECT is_embed FROM file_data WHERE id = ?`, ids[0])
	if err := row.Scan(&isEmbed); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if isEmbed {
		t.Errorf("is_embed = true, want false (stale embedding must not apply)")
	}
}

func TestUpdateEmbedding_DismissedSkipped(t *testing.T) {
	db := openPerDB(t)
	ids := seedFileData(t, db, "/a", 1, []string{"alpha"})
	if err := databaseHandler.Dismiss(db, context.Background(), "/a"); err != nil {
		t.Fatalf("Dismiss: %v", err)
	}

	applied, err := databaseHandler.UpdateEmbedding(db, context.Background(),
		[]databaseHandler.EmbeddingItem{{ID: ids[0], Content: "alpha", Embedding: []byte{0x01}}})
	if err != nil {
		t.Fatalf("UpdateEmbedding: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("applied = %v, want empty (dismissed row must skip)", applied)
	}
}

func TestUpdateEmbedding_UnknownIDSkipped(t *testing.T) {
	db := openPerDB(t)
	seedFileData(t, db, "/a", 1, []string{"alpha"})

	applied, err := databaseHandler.UpdateEmbedding(db, context.Background(),
		[]databaseHandler.EmbeddingItem{{ID: 999_999, Content: "alpha", Embedding: []byte{0x01}}})
	if err != nil {
		t.Fatalf("UpdateEmbedding: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("applied = %v, want empty (missing ID must skip)", applied)
	}
}

func TestUpdateEmbedding_PartialApply(t *testing.T) {
	db := openPerDB(t)
	ids := seedFileData(t, db, "/a", 2, []string{"alpha", "beta"})

	items := []databaseHandler.EmbeddingItem{
		{ID: ids[0], Content: "alpha", Embedding: []byte{0xAA}},
		{ID: ids[1], Content: "WRONG", Embedding: []byte{0xBB}}, // skipped
	}
	applied, err := databaseHandler.UpdateEmbedding(db, context.Background(), items)
	if err != nil {
		t.Fatalf("UpdateEmbedding: %v", err)
	}
	if len(applied) != 1 || applied[0] != ids[0] {
		t.Errorf("applied = %v, want [%d]", applied, ids[0])
	}
}
