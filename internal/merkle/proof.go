package merkle

import (
	"errors"
	"fmt"
)

// InclusionProof returns the audit path (sibling hashes) for leaves[index].
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

func collectPath(nodes [][32]byte, index int, path *[][32]byte) {
	if len(nodes) == 1 {
		return
	}
	mid := splitPoint(len(nodes))
	if index < mid {
		sibling := buildSubtree(nodes[mid:])
		*path = append(*path, sibling)
		collectPath(nodes[:mid], index, path)
	} else {
		sibling := buildSubtree(nodes[:mid])
		*path = append(*path, sibling)
		collectPath(nodes[mid:], index-mid, path)
	}
}

// splitPoint returns the number of leaves in the left subtree for a tree of size n.
func splitPoint(n int) int {
	k := 1
	for k < n {
		k <<= 1
	}
	return k >> 1
}

// VerifyInclusion verifies that leafHash is in a tree of treeSize leaves with the
// given root, at the given index, using the provided audit path.
func VerifyInclusion(leafHash, root [32]byte, index, treeSize int, proof [][32]byte) bool {
	if treeSize == 0 || index < 0 || index >= treeSize {
		return false
	}
	computed := recomputeRoot(leafHash, index, treeSize, proof)
	return computed == root
}

func recomputeRoot(hash [32]byte, index, size int, proof [][32]byte) [32]byte {
	for _, sibling := range proof {
		mid := splitPoint(size)
		if index < mid {
			hash = HashInternal(hash, sibling)
			size = mid
		} else {
			hash = HashInternal(sibling, hash)
			index -= mid
			size -= mid
		}
	}
	return hash
}
