package test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/yasserrmd/siqlah/internal/merkle"
)

func makeLeaves(n int) [][32]byte {
	leaves := make([][32]byte, n)
	for i := 0; i < n; i++ {
		data := fmt.Sprintf("leaf-%d", i)
		h := sha256.Sum256([]byte(data))
		leaves[i] = merkle.HashLeaf(h[:])
	}
	return leaves
}

func TestAllInclusionProofsSmallTree(t *testing.T) {
	for treeSize := 1; treeSize <= 20; treeSize++ {
		leaves := makeLeaves(treeSize)
		root := merkle.BuildRoot(leaves)

		for i := 0; i < treeSize; i++ {
			path, err := merkle.InclusionProof(leaves, i)
			if err != nil {
				t.Errorf("tree=%d leaf=%d: InclusionProof error: %v", treeSize, i, err)
				continue
			}
			if !merkle.VerifyInclusion(leaves[i], root, i, treeSize, path) {
				t.Errorf("tree=%d leaf=%d: VerifyInclusion failed", treeSize, i)
			}
		}
	}
}

func TestTamperedLeafFailsProof(t *testing.T) {
	leaves := makeLeaves(8)
	root := merkle.BuildRoot(leaves)

	// Get proof for leaf 3.
	path, err := merkle.InclusionProof(leaves, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Use a different leaf hash — should fail.
	wrongLeaf := makeLeaves(1)[0]
	if merkle.VerifyInclusion(wrongLeaf, root, 3, 8, path) {
		t.Error("expected VerifyInclusion to fail with tampered leaf")
	}
}

func TestWrongRootFailsProof(t *testing.T) {
	leaves := makeLeaves(4)
	root := merkle.BuildRoot(leaves)

	path, err := merkle.InclusionProof(leaves, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Use wrong root.
	var wrongRoot [32]byte
	wrongRoot[0] = 0xff
	if merkle.VerifyInclusion(leaves[0], wrongRoot, 0, 4, path) {
		t.Error("expected VerifyInclusion to fail with wrong root")
	}

	// Correct root should still pass.
	if !merkle.VerifyInclusion(leaves[0], root, 0, 4, path) {
		t.Error("correct root should pass verification")
	}
}

func TestConsistencyProofSmallTrees(t *testing.T) {
	for newSize := 2; newSize <= 16; newSize++ {
		leaves := makeLeaves(newSize)
		newRoot := merkle.BuildRoot(leaves)

		for oldSize := 1; oldSize < newSize; oldSize++ {
			oldRoot := merkle.BuildRoot(leaves[:oldSize])

			proof, err := merkle.ConsistencyProof(oldSize, newSize, leaves)
			if err != nil {
				t.Errorf("old=%d new=%d: ConsistencyProof error: %v", oldSize, newSize, err)
				continue
			}

			if !merkle.VerifyConsistency(oldRoot, newRoot, oldSize, newSize, proof) {
				t.Errorf("old=%d new=%d: VerifyConsistency failed", oldSize, newSize)
			}
		}
	}
}

func TestConsistencyProofTamperedOldRoot(t *testing.T) {
	leaves := makeLeaves(8)
	newRoot := merkle.BuildRoot(leaves)
	oldRoot := merkle.BuildRoot(leaves[:4])

	proof, err := merkle.ConsistencyProof(4, 8, leaves)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with old root.
	var wrongOldRoot [32]byte
	wrongOldRoot[0] = 0xde
	wrongOldRoot[1] = 0xad
	if merkle.VerifyConsistency(wrongOldRoot, newRoot, 4, 8, proof) {
		t.Error("expected VerifyConsistency to fail with tampered old root")
	}

	// Correct should pass.
	if !merkle.VerifyConsistency(oldRoot, newRoot, 4, 8, proof) {
		t.Error("correct consistency proof should pass")
	}
}

func TestInclusionProofLargeTree(t *testing.T) {
	const treeSize = 1000
	leaves := makeLeaves(treeSize)
	root := merkle.BuildRoot(leaves)

	// Test a sample of indices.
	indices := []int{0, 1, 499, 500, 998, 999}
	for _, i := range indices {
		path, err := merkle.InclusionProof(leaves, i)
		if err != nil {
			t.Errorf("leaf=%d: %v", i, err)
			continue
		}
		if !merkle.VerifyInclusion(leaves[i], root, i, treeSize, path) {
			t.Errorf("leaf=%d: verification failed", i)
		}
	}
}

func TestConsistencyProofDoubleSizes(t *testing.T) {
	// Verify consistency between a tree of size N and size 2N.
	for n := 1; n <= 8; n++ {
		old := makeLeaves(n)
		combined := makeLeaves(2 * n)

		oldRoot := merkle.BuildRoot(old)
		newRoot := merkle.BuildRoot(combined)

		proof, err := merkle.ConsistencyProof(n, 2*n, combined)
		if err != nil {
			t.Errorf("n=%d: %v", n, err)
			continue
		}
		if !merkle.VerifyConsistency(oldRoot, newRoot, n, 2*n, proof) {
			t.Errorf("n=%d: consistency proof verification failed", n)
		}
	}
}
