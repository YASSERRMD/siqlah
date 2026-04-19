package merkle

import (
	"errors"
	"fmt"
)

// ConsistencyProof returns the hashes needed to prove that the tree of oldSize
// leaves is a prefix of the tree of newSize leaves (RFC 6962 §2.2).
// Proof elements are ordered inner-to-outer (leaf-level subtrees first).
func ConsistencyProof(oldSize, newSize int, leaves [][32]byte) ([][32]byte, error) {
	if oldSize <= 0 || newSize <= 0 {
		return nil, errors.New("tree sizes must be positive")
	}
	if oldSize > newSize {
		return nil, fmt.Errorf("oldSize %d > newSize %d", oldSize, newSize)
	}
	if newSize > len(leaves) {
		return nil, fmt.Errorf("newSize %d exceeds leaf count %d", newSize, len(leaves))
	}
	if oldSize == newSize {
		return nil, nil
	}
	var proof [][32]byte
	collectConsistency(oldSize, newSize, leaves, true, &proof)
	return proof, nil
}

// collectConsistency implements SUBPROOF from RFC 6962 §2.2, appending
// proof elements inner-to-outer.
func collectConsistency(m, n int, leaves [][32]byte, b bool, proof *[][32]byte) {
	if m == n {
		if !b {
			*proof = append(*proof, buildSubtree(leaves[:n]))
		}
		return
	}
	k := splitPoint(n)
	if m <= k {
		collectConsistency(m, k, leaves[:k], b, proof)
		*proof = append(*proof, buildSubtree(leaves[k:n]))
	} else {
		collectConsistency(m-k, n-k, leaves[k:n], false, proof)
		*proof = append(*proof, buildSubtree(leaves[:k]))
	}
}

// VerifyConsistency checks that oldRoot (tree of oldSize) is a prefix of newRoot
// (tree of newSize) given the consistency proof.
func VerifyConsistency(oldRoot, newRoot [32]byte, oldSize, newSize int, proof [][32]byte) bool {
	if oldSize == newSize {
		return oldRoot == newRoot && len(proof) == 0
	}
	if oldSize == 0 || newSize == 0 || oldSize > newSize {
		return false
	}
	pos := 0
	gotOld, gotNew, ok := verifySubproof(oldSize, newSize, true, oldRoot, proof, &pos)
	return ok && pos == len(proof) && gotOld == oldRoot && gotNew == newRoot
}

// verifySubproof mirrors collectConsistency: it consumes proof elements and
// returns the reconstructed old-subtree hash and new-subtree hash.
func verifySubproof(m, n int, b bool, knownRoot [32]byte, proof [][32]byte, pos *int) ([32]byte, [32]byte, bool) {
	var zero [32]byte
	if m == n {
		if b {
			// The subtree D[0:m] is identical in both trees; its hash is knownRoot.
			return knownRoot, knownRoot, true
		}
		if *pos >= len(proof) {
			return zero, zero, false
		}
		h := proof[*pos]
		(*pos)++
		return h, h, true
	}
	k := splitPoint(n)
	if m <= k {
		leftOld, leftNew, ok := verifySubproof(m, k, b, knownRoot, proof, pos)
		if !ok {
			return zero, zero, false
		}
		if *pos >= len(proof) {
			return zero, zero, false
		}
		rightHash := proof[*pos]
		(*pos)++
		return leftOld, HashInternal(leftNew, rightHash), true
	}
	rightOld, rightNew, ok := verifySubproof(m-k, n-k, false, zero, proof, pos)
	if !ok {
		return zero, zero, false
	}
	if *pos >= len(proof) {
		return zero, zero, false
	}
	leftHash := proof[*pos]
	(*pos)++
	return HashInternal(leftHash, rightOld), HashInternal(leftHash, rightNew), true
}
