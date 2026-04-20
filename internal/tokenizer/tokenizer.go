// Package tokenizer wraps the Rust siqlah_tokenizer CDylib via CGo.
// If the shared library is unavailable, all operations return an unverified
// result with Verified=false and a warning, rather than failing hard.
package tokenizer

// #cgo LDFLAGS: -L${SRCDIR}/../../tokenizer-rs/target/release -lsiqlah_tokenizer -ldl -lm -Wl,-rpath,${SRCDIR}/../../tokenizer-rs/target/release
// #include <stdlib.h>
// extern char* siqlah_tokenize(const char* text, const char* tokenizer_json);
// extern void  siqlah_free(char* ptr);
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// TokenizeResult holds output from the Rust tokenizer.
type TokenizeResult struct {
	TokenCount        int    `json:"token_count"`
	BoundaryRootHex   string `json:"boundary_root_hex"`
	TokenizerHash     string `json:"tokenizer_hash"`
	Verified          bool
}

// rustResult matches the JSON returned by siqlah_tokenize.
type rustResult struct {
	TokenCount      int    `json:"token_count"`
	BoundaryRootHex string `json:"boundary_root_hex"`
	TokenizerHash   string `json:"tokenizer_hash"`
}

// Tokenize tokenizes text using the provided tokenizer JSON.
// On success, Verified is true. On any error (including missing .so),
// returns an empty unverified result.
func Tokenize(text, tokenizerJSON string) (*TokenizeResult, error) {
	cText := C.CString(text)
	cJSON := C.CString(tokenizerJSON)
	defer C.free(unsafe.Pointer(cText))
	defer C.free(unsafe.Pointer(cJSON))

	raw := C.siqlah_tokenize(cText, cJSON)
	if raw == nil {
		return unverified(), fmt.Errorf("siqlah_tokenize returned nil")
	}
	defer C.siqlah_free(raw)

	var rr rustResult
	if err := json.Unmarshal([]byte(C.GoString(raw)), &rr); err != nil {
		return unverified(), fmt.Errorf("unmarshal result: %w", err)
	}
	return &TokenizeResult{
		TokenCount:      rr.TokenCount,
		BoundaryRootHex: rr.BoundaryRootHex,
		TokenizerHash:   rr.TokenizerHash,
		Verified:        true,
	}, nil
}

func unverified() *TokenizeResult {
	return &TokenizeResult{Verified: false}
}
