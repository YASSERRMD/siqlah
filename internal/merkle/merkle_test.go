package merkle_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/yasserrmd/siqlah/internal/merkle"
)

func makeLeaves(n int) [][32]byte {
	leaves := make([][32]byte, n)
	for i := range leaves {
		leaves[i] = merkle.HashLeaf([]byte(fmt.Sprintf("leaf-%d", i)))
	}
	return leaves
}

func TestBuildRoot_Sizes(t *testing.T) {
	sizes := []int{1, 2, 3, 4, 5, 7, 8, 16, 17, 100, 257}
	for _, n := range sizes {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			leaves := makeLeaves(n)
			root := merkle.BuildRoot(leaves)
			if root == ([32]byte{}) {
				t.Errorf("got zero root for n=%d", n)
			}
		})
	}
}

func TestBuildRoot_Determinism(t *testing.T) {
	leaves := makeLeaves(100)
	r1 := merkle.BuildRoot(leaves)
	r2 := merkle.BuildRoot(leaves)
	if r1 != r2 {
		t.Fatal("BuildRoot is not deterministic")
	}
}

func TestBuildRoot_Empty(t *testing.T) {
	root := merkle.BuildRoot(nil)
	if root != ([32]byte{}) {
		t.Fatal("expected zero root for empty tree")
	}
}

func TestInclusionProof_AllLeaves(t *testing.T) {
	sizes := []int{1, 2, 3, 4, 5, 7, 8, 16, 17, 100, 257}
	for _, n := range sizes {
		leaves := makeLeaves(n)
		root := merkle.BuildRoot(leaves)
		for i := 0; i < n; i++ {
			proof, err := merkle.InclusionProof(leaves, i)
			if err != nil {
				t.Errorf("n=%d i=%d: InclusionProof error: %v", n, i, err)
				continue
			}
			if !merkle.VerifyInclusion(leaves[i], root, i, n, proof) {
				t.Errorf("n=%d i=%d: VerifyInclusion failed", n, i)
			}
		}
	}
}

func TestInclusionProof_TamperDetection(t *testing.T) {
	leaves := makeLeaves(8)
	root := merkle.BuildRoot(leaves)
	proof, _ := merkle.InclusionProof(leaves, 3)

	// Tamper the leaf.
	var badLeaf [32]byte
	h := sha256.Sum256([]byte("tampered"))
	copy(badLeaf[:], h[:])

	if merkle.VerifyInclusion(badLeaf, root, 3, 8, proof) {
		t.Fatal("expected verification to fail for tampered leaf")
	}
}

func TestConsistencyProof(t *testing.T) {
	sizes := [][2]int{{1, 2}, {2, 4}, {3, 7}, {4, 8}, {4, 16}, {7, 17}, {8, 16}, {100, 257}}
	for _, pair := range sizes {
		old, new := pair[0], pair[1]
		leaves := makeLeaves(new)
		oldRoot := merkle.BuildRoot(leaves[:old])
		newRoot := merkle.BuildRoot(leaves)

		proof, err := merkle.ConsistencyProof(old, new, leaves)
		if err != nil {
			t.Errorf("old=%d new=%d: ConsistencyProof error: %v", old, new, err)
			continue
		}
		if !merkle.VerifyConsistency(oldRoot, newRoot, old, new, proof) {
			t.Errorf("old=%d new=%d: VerifyConsistency failed", old, new)
		}
	}
}

func TestConsistencyProof_SameSize(t *testing.T) {
	leaves := makeLeaves(8)
	root := merkle.BuildRoot(leaves)
	proof, err := merkle.ConsistencyProof(8, 8, leaves)
	if err != nil {
		t.Fatal(err)
	}
	if !merkle.VerifyConsistency(root, root, 8, 8, proof) {
		t.Fatal("same-size consistency should verify")
	}
}

func BenchmarkBuildRoot_10k(b *testing.B) {
	leaves := makeLeaves(10_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		merkle.BuildRoot(leaves)
	}
}

func BenchmarkBuildRoot_100k(b *testing.B) {
	leaves := makeLeaves(100_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		merkle.BuildRoot(leaves)
	}
}
