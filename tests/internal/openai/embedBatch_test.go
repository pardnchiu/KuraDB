package openai_integration_test

import (
	"math"
	"testing"

	"github.com/pardnchiu/KuraDB/internal/openai"
)

// EmbedBatch requires a live OpenAI API endpoint and credentials, so only the
// pure codec helpers Encode and Decode are covered here.

func TestEncode(t *testing.T) {
	tests := []struct {
		name   string
		v      openai.Vector
		wantBy int
	}{
		{"nil vector", nil, 0},
		{"empty vector", openai.Vector{}, 0},
		{"single float", openai.Vector{1.5}, 4},
		{"multi float", openai.Vector{0, 1, -1, 3.14}, 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := openai.Encode(tt.v); len(got) != tt.wantBy {
				t.Errorf("Encode() len = %d, want %d", len(got), tt.wantBy)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		b       []byte
		wantErr bool
		wantLen int
	}{
		{"nil bytes", nil, false, 0},
		{"empty bytes", []byte{}, false, 0},
		{"non-multiple-of-4", []byte{1, 2, 3}, true, 0},
		{"single float", openai.Encode(openai.Vector{2.5}), false, 1},
		{"multi float", openai.Encode(openai.Vector{1, -2, 3, -4}), false, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := openai.Decode(tt.b)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decode() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("Decode() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	inputs := []openai.Vector{
		{1, 2, 3},
		{0, 0, 0},
		{-1.5, 2.25, 100, -99.5},
		{math.MaxFloat32, -math.MaxFloat32, 0, 1},
	}
	for _, v := range inputs {
		got, err := openai.Decode(openai.Encode(v))
		if err != nil {
			t.Fatalf("round trip Decode err: %v", err)
		}
		if len(got) != len(v) {
			t.Fatalf("round trip len: got %d, want %d", len(got), len(v))
		}
		for i := range v {
			if got[i] != v[i] {
				t.Errorf("round trip [%d]: got %v, want %v", i, got[i], v[i])
			}
		}
	}
}
