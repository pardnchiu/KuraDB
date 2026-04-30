package openai_integration_test

import (
	"testing"

	"github.com/pardnchiu/KuraDB/internal/openai"
)

func TestDim(t *testing.T) {
	if got := openai.Dim(); got <= 0 {
		t.Errorf("Dim() = %d, want positive", got)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantErr bool
	}{
		{"missing api key", "", true},
		{"valid api key", "sk-dummy-test-key", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAI_API_KEY", tt.envVal)
			got, err := openai.New()
			if (err != nil) != tt.wantErr {
				t.Fatalf("New() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && got == nil {
				t.Error("New() returned nil with no error")
			}
		})
	}
}
