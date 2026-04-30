package openai_integration_test

import (
	"sync"
	"testing"

	"github.com/pardnchiu/KuraDB/internal/openai"
)

func TestNewCache(t *testing.T) {
	c := openai.NewCache()
	if c == nil {
		t.Fatal("NewCache() returned nil")
	}
	if c.Len() != 0 {
		t.Errorf("Len() on fresh cache = %d, want 0", c.Len())
	}
}

func TestCache_SetGet(t *testing.T) {
	tests := []struct {
		name string
		q    string
		v    []float32
	}{
		{"nominal", "hello", []float32{1, 2, 3}},
		{"empty query", "", []float32{0}},
		{"empty vector", "k", []float32{}},
		{"nil vector", "n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := openai.NewCache()
			c.Set(tt.q, tt.v)
			got, ok := c.Get(tt.q)
			if !ok {
				t.Fatalf("Get(%q) miss after Set", tt.q)
			}
			if len(got) != len(tt.v) {
				t.Errorf("Get(%q) len = %d, want %d", tt.q, len(got), len(tt.v))
			}
		})
	}
}

func TestCache_GetMiss(t *testing.T) {
	c := openai.NewCache()
	if v, ok := c.Get("not-set"); ok || v != nil {
		t.Errorf("Get on empty cache = (%v, %v), want (nil, false)", v, ok)
	}
}

func TestCache_Preload(t *testing.T) {
	var fired bool
	c := openai.NewCache()
	c.OnSet(func(q string, v []float32) { fired = true })
	c.Preload("k", []float32{1})
	if fired {
		t.Error("Preload should not fire OnSet callback")
	}
	got, ok := c.Get("k")
	if !ok || len(got) != 1 {
		t.Errorf("Preload not visible via Get: ok=%v len=%d", ok, len(got))
	}
}

func TestCache_OnSet(t *testing.T) {
	var (
		mu   sync.Mutex
		seen []string
	)
	c := openai.NewCache()
	c.OnSet(func(q string, v []float32) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, q)
	})
	c.Set("a", []float32{1})
	c.Set("b", []float32{2})

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("OnSet fired %d times, want 2", len(seen))
	}
}

func TestCache_Len(t *testing.T) {
	c := openai.NewCache()
	if c.Len() != 0 {
		t.Errorf("Len() initial = %d, want 0", c.Len())
	}
	c.Set("a", []float32{1})
	if c.Len() != 1 {
		t.Errorf("Len() after 1 Set = %d, want 1", c.Len())
	}
	c.Set("a", []float32{2})
	if c.Len() != 1 {
		t.Errorf("Len() after overwrite = %d, want 1", c.Len())
	}
	c.Preload("b", []float32{3})
	if c.Len() != 2 {
		t.Errorf("Len() after Preload = %d, want 2", c.Len())
	}
}

func TestCache_NilSafe(t *testing.T) {
	var c *openai.Cache
	if got := c.Len(); got != 0 {
		t.Errorf("nil.Len() = %d, want 0", got)
	}
	if v, ok := c.Get("x"); v != nil || ok {
		t.Errorf("nil.Get() = (%v, %v), want (nil, false)", v, ok)
	}
	c.Set("x", []float32{1})
	c.Preload("x", []float32{1})
	c.OnSet(func(q string, v []float32) {})
}
