package tokenizer_test

import (
	"testing"

	"github.com/yasserrmd/siqlah/internal/tokenizer"
)

// TestGracefulDegradation ensures the wrapper returns an unverified result
// when called with invalid tokenizer JSON (no Rust lib needed for this test).
func TestGracefulDegradation(t *testing.T) {
	result, err := tokenizer.Tokenize("hello world", "not valid json")
	if err == nil {
		t.Log("tokenizer succeeded (Rust lib available)")
		if result.Verified {
			t.Log("verified=true, Rust FFI working")
		}
		return
	}
	// Error is expected when tokenizer JSON is invalid.
	if result == nil {
		t.Fatal("expected non-nil result even on error")
	}
	if result.Verified {
		t.Error("expected Verified=false on error")
	}
}
