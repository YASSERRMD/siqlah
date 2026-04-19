package merkle

// DEPRECATED: Use Tessera backend for production (--log-backend=tessera).
// This package is retained for the SQLite legacy backend and testing environments without Tessera.

import (
	"errors"
	"fmt"
)

// InclusionProof returns the audit path for leaves[index].
// Proof is ordered from leaf level (innermost) to root level (outermost).
func InclusionProof(leaves [][32]byte, index int) ([][32]byte, error) {
	if len(leaves) == 0 {
		return nil, errors.New("empty tree")
	}
	if index < 0 || index >= len(leaves) {
		return nil, fmt.Errorf("index %d out of range [0, %d)", index, len(leaves))
	}
	var path [][32]byte
	collectPath(leaves, index, &path)
	return path, nil
}

// collectPath appends siblings from leaf level to root level (inner to outer).
func collectPath(nodes [][32]byte, index int, path *[][32]byte) {
	if len(nodes) == 1 {
		return
	}
	k := splitPoint(len(nodes))
	if index < k {
		collectPath(nodes[:k], index, path)
		*path = append(*path, buildSubtree(nodes[k:]))
	} else {
		collectPath(nodes[k:], index-k, path)
		*path = append(*path, buildSubtree(nodes[:k]))
	}
}

// splitPoint returns the largest power of 2 strictly less than n.
func splitPoint(n int) int {
	k := 1
	for k < n {
		k <<= 1
	}
	return k >> 1
}

// VerifyInclusion verifies that leafHash is at index in a tree of treeSize leaves
// with the given root, using the provided audit path (inner-to-outer order).
func VerifyInclusion(leafHash, root [32]byte, index, treeSize int, proof [][32]byte) bool {
	if treeSize == 0 || index < 0 || index >= treeSize {
		return false
	}

	// Precompute top-down directions; proof is bottom-up, so we apply in reverse.
	goRight := make([]bool, len(proof))
	n, i := treeSize, index
	for j := 0; j < len(proof); j++ {
		k := splitPoint(n)
		if i < k {
			goRight[j] = false
			n = k
		} else {
			goRight[j] = true
			i -= k
			n -= k
		}
	}

	// Apply proof[0..] bottom-up using directions in reverse (outer-in → inner-out).
	hash := leafHash
	for j := 0; j < len(proof); j++ {
		d := goRight[len(proof)-1-j]
		if d {
			hash = HashInternal(proof[j], hash)
		} else {
			hash = HashInternal(hash, proof[j])
		}
	}
	return hash == root
}
