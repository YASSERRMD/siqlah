package merkle

import (
	"encoding/hex"
	"fmt"
)

// FormatRoot returns the hex-encoded string of a Merkle root.
//
// Deprecated: Use the Tessera backend (--log-backend=tessera) for production use.
func FormatRoot(r [32]byte) string {
	return hex.EncodeToString(r[:])
}

// ParseRoot decodes a hex string into a 32-byte Merkle root.
//
// Deprecated: Use the Tessera backend (--log-backend=tessera) for production use.
func ParseRoot(s string) ([32]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return [32]byte{}, fmt.Errorf("parse root: %w", err)
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("parse root: expected 32 bytes, got %d", len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return out, nil
}
