package checkpoint_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/store"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func openStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func genKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func insertReceipts(t *testing.T, s store.Store, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		r := vur.Receipt{
			ID: fmt.Sprintf("rec-%03d", i), Version: "1.0.0",
			Provider: "openai", Model: "gpt-4o",
			InputTokens: int64(100 + i), Timestamp: time.Now().UTC(),
		}
		if _, err := s.AppendReceipt(r); err != nil {
			t.Fatalf("append receipt: %v", err)
		}
	}
}

func TestBuildSignRoundTrip(t *testing.T) {
	s := openStore(t)
	opPub, opPriv := genKey(t)
	insertReceipts(t, s, 10)

	b := checkpoint.NewBuilder(s, opPriv, 1000)
	cp, err := b.BuildAndSign()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if cp == nil {
		t.Fatal("expected non-nil checkpoint")
	}
	if cp.TreeSize != 10 {
		t.Errorf("tree size: got %d, want 10", cp.TreeSize)
	}
	if err := checkpoint.VerifyOperatorSignature(*cp, opPub); err != nil {
		t.Fatalf("verify operator sig: %v", err)
	}
}

func TestNoReceiptsReturnsNil(t *testing.T) {
	s := openStore(t)
	_, opPriv := genKey(t)
	b := checkpoint.NewBuilder(s, opPriv, 1000)
	cp, err := b.BuildAndSign()
	if err != nil {
		t.Fatal(err)
	}
	if cp != nil {
		t.Fatal("expected nil checkpoint for empty store")
	}
}

func TestChainConsistency(t *testing.T) {
	s := openStore(t)
	_, opPriv := genKey(t)
	b := checkpoint.NewBuilder(s, opPriv, 5)

	insertReceipts(t, s, 5)
	cp1, err := b.BuildAndSign()
	if err != nil || cp1 == nil {
		t.Fatalf("build cp1: %v", err)
	}
	insertReceipts(t, s, 5)
	cp2, err := b.BuildAndSign()
	if err != nil || cp2 == nil {
		t.Fatalf("build cp2: %v", err)
	}
	if err := checkpoint.VerifyChainConsistency(*cp1, *cp2); err != nil {
		t.Fatalf("chain consistency: %v", err)
	}
}

func TestWitnessCosignAndVerify(t *testing.T) {
	s := openStore(t)
	opPub, opPriv := genKey(t)
	wPub, wPriv := genKey(t)
	insertReceipts(t, s, 3)

	b := checkpoint.NewBuilder(s, opPriv, 1000)
	cp, _ := b.BuildAndSign()

	sigHex, err := checkpoint.CoSign(*cp, opPub, wPriv)
	if err != nil {
		t.Fatalf("cosign: %v", err)
	}
	if err := checkpoint.VerifyWitness(*cp, wPub, sigHex); err != nil {
		t.Fatalf("verify witness: %v", err)
	}
}

func TestRefuseCosignBadOperator(t *testing.T) {
	s := openStore(t)
	_, opPriv := genKey(t)
	wrongPub, _ := genKey(t)
	_, wPriv := genKey(t)
	insertReceipts(t, s, 3)

	b := checkpoint.NewBuilder(s, opPriv, 1000)
	cp, _ := b.BuildAndSign()

	if _, err := checkpoint.CoSign(*cp, wrongPub, wPriv); err == nil {
		t.Fatal("expected cosign to fail with wrong operator key")
	}
}
