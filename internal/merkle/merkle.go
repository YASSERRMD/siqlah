package merkle

import "crypto/sha256"

// RFC 6962 domain separation prefixes.
const (
	leafPrefix     = byte(0x00)
	internalPrefix = byte(0x01)
)

// HashLeaf returns the RFC 6962 leaf hash of data.
func HashLeaf(data []byte) [32]byte {
	h := sha256.New()
	h.Write([]byte{leafPrefix})
	h.Write(data)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// HashInternal returns the RFC 6962 internal node hash.
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
