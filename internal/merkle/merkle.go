// Package merkle provides an RFC 6962-style Merkle tree implementation.
//
// Deprecated: For production deployments, use the Tessera log backend
// (internal/tessera and internal/store/tessera_store.go) which provides
// the same cryptographic guarantees with production-grade performance
// and native C2SP witness support. This package is retained for:
//   - Testing and development without Tessera dependencies
//   - Environments where the SQLite-only backend is preferred
//   - Reference implementation for understanding the cryptographic primitives
//
// New features should target the Tessera backend. This package will not
// receive new features but will continue to receive bug fixes.
// Enable with: --log-backend=tessera
package merkle

import "crypto/sha256"

// RFC 6962 domain separation prefixes.
const (
	leafPrefix     = byte(0x00)
	internalPrefix = byte(0x01)
)

// HashLeaf returns the RFC 6962 leaf hash of data.
//
// Deprecated: Use the Tessera backend (--log-backend=tessera) for production use.
func HashLeaf(data []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte{leafPrefix})
	h.Write(data)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// HashInternal returns the RFC 6962 internal node hash.
//
// Deprecated: Use the Tessera backend (--log-backend=tessera) for production use.
func HashInternal(left, right [32]byte) [32]byte {
	h := sha256.New()
	h.Write([]byte{internalPrefix})
	h.Write(left[:])
	h.Write(right[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// BuildRoot builds the Merkle root from a slice of leaf hashes.
// Odd nodes are promoted without re-hashing (RFC 6962 §2.1).
// Returns zero hash for an empty tree.
//
// Deprecated: Use the Tessera backend (--log-backend=tessera) for production use.
func BuildRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 0 {
		return [32]byte{}
	}
	return buildSubtree(leaves)
}

func buildSubtree(nodes [][32]byte) [32]byte {
	if len(nodes) == 1 {
		return nodes[0]
	}
	mid := splitPoint(len(nodes))
	left := buildSubtree(nodes[:mid])
	right := buildSubtree(nodes[mid:])
	return HashInternal(left, right)
}
