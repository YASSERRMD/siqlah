package merkle

import (
	"errors"
	"fmt"
)

// ConsistencyProof returns the hashes needed to prove that the tree of oldSize
// leaves is a prefix of the tree of newSize leaves (RFC 6962 §2.2).
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

func collectConsistency(m, n int, leaves [][32]byte, startFromScratch bool, proof *[][32]byte) {
	if m == n {
		if !startFromScratch {
			*proof = append(*proof, buildSubtree(leaves[:n]))
		}
		return
	}
	k := splitPoint(n)
	if m <= k {
		collectConsistency(m, k, leaves[:k], startFromScratch, proof)
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

	var left, right [32]byte
	node := oldSize - 1
	i := 0

	// Determine starting hash.
	if isPow2(oldSize) {
		left = oldRoot
		right = oldRoot
	} else {
		if i >= len(proof) {
			return false
		}
		left = proof[i]
		right = proof[i]
		i++
	}

	for j := i; j < len(proof); j++ {
		sibling := proof[j]
		if node%2 == 1 {
			left = HashInternal(sibling, left)
			right = HashInternal(sibling, right)
		} else if node < newSize-1 {
			right = HashInternal(right, sibling)
		}
		node = (node - 1) / 2
	}
	return left == oldRoot && right == newRoot
}

func isPow2(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}
