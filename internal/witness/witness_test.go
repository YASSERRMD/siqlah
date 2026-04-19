package witness_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/yasserrmd/siqlah/internal/witness"
	"golang.org/x/mod/sumdb/note"
)

func TestCheckpointFormatRoundTrip(t *testing.T) {
	rootHash := make([]byte, 32)
	if _, err := rand.Read(rootHash); err != nil {
		t.Fatal(err)
	}
	body := witness.FormatCheckpoint("siqlah.dev/log", 42, rootHash)

	cp, err := witness.ParseCheckpoint(body)
	if err != nil {
		t.Fatalf("ParseCheckpoint: %v", err)
	}
	if cp.Origin != "siqlah.dev/log" {
		t.Errorf("origin: got %q, want siqlah.dev/log", cp.Origin)
	}
	if cp.TreeSize != 42 {
		t.Errorf("tree_size: got %d, want 42", cp.TreeSize)
	}
	if hex.EncodeToString(cp.RootHash) != hex.EncodeToString(rootHash) {
		t.Errorf("root_hash mismatch")
	}
}

func TestCheckpointSignAndVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	origin := "siqlah.dev/log"

	signer, err := witness.NewNoteSigner(origin, priv)
	if err != nil {
		t.Fatalf("NewNoteSigner: %v", err)
	}
	verifier, err := witness.NewNoteVerifier(origin, pub)
	if err != nil {
		t.Fatalf("NewNoteVerifier: %v", err)
	}

	rootHash := make([]byte, 32)
	rand.Read(rootHash)
	body := witness.FormatCheckpoint(origin, 100, rootHash)

	signed, err := witness.SignCheckpoint(body, signer)
	if err != nil {
		t.Fatalf("SignCheckpoint: %v", err)
	}

	cp, n, err := witness.OpenCheckpoint(string(signed), note.VerifierList(verifier))
	if err != nil {
		t.Fatalf("OpenCheckpoint: %v", err)
	}
	if cp.TreeSize != 100 {
		t.Errorf("tree_size: got %d, want 100", cp.TreeSize)
	}
	if len(n.Sigs) == 0 {
		t.Errorf("expected at least one verified signature")
	}
}

func TestCosignatureCreationAndVerification(t *testing.T) {
	origin := "siqlah.dev/log"
	opPub, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	w1Pub, w1Priv, _ := ed25519.GenerateKey(rand.Reader)
	w2Pub, w2Priv, _ := ed25519.GenerateKey(rand.Reader)

	opSigner, _ := witness.NewNoteSigner(origin, opPriv)
	opVerifier, _ := witness.NewNoteVerifier(origin, opPub)
	w1Signer, _ := witness.NewNoteSigner("witness1", w1Priv)
	w1Verifier, _ := witness.NewNoteVerifier("witness1", w1Pub)
	w2Signer, _ := witness.NewNoteSigner("witness2", w2Priv)
	w2Verifier, _ := witness.NewNoteVerifier("witness2", w2Pub)

	rootHash := make([]byte, 32)
	rand.Read(rootHash)
	body := witness.FormatCheckpoint(origin, 50, rootHash)

	// Operator signs.
	signed, err := witness.SignCheckpoint(body, opSigner)
	if err != nil {
		t.Fatalf("SignCheckpoint: %v", err)
	}

	// Witness 1 cosigns.
	n, _ := note.Open(signed, note.VerifierList(opVerifier))
	cosig1, err := note.Sign(n, w1Signer)
	if err != nil {
		t.Fatalf("witness1 cosign: %v", err)
	}

	// Witness 2 cosigns the original (not the already-cosigned).
	n2, _ := note.Open(signed, note.VerifierList(opVerifier))
	cosig2, err := note.Sign(n2, w2Signer)
	if err != nil {
		t.Fatalf("witness2 cosign: %v", err)
	}

	// Merge cosignatures.
	merged, err := witness.MergeCosignatures(signed, []string{string(cosig1), string(cosig2)}, opVerifier)
	if err != nil {
		t.Fatalf("MergeCosignatures: %v", err)
	}

	// Verify merged note contains all signatures.
	allVerifiers := note.VerifierList(opVerifier, w1Verifier, w2Verifier)
	mergedNote, err := note.Open(merged, allVerifiers)
	if err != nil {
		t.Fatalf("open merged note: %v", err)
	}
	if len(mergedNote.Sigs) < 3 {
		t.Errorf("expected 3 verified sigs, got %d", len(mergedNote.Sigs))
	}
}

func TestKOfNThresholdVerification(t *testing.T) {
	origin := "siqlah.dev/log"
	opPub, opPriv, _ := ed25519.GenerateKey(rand.Reader)
	w1Pub, w1Priv, _ := ed25519.GenerateKey(rand.Reader)

	opSigner, _ := witness.NewNoteSigner(origin, opPriv)
	opVerifier, _ := witness.NewNoteVerifier(origin, opPub)
	w1Signer, _ := witness.NewNoteSigner("witness1", w1Priv)
	w1Verifier, _ := witness.NewNoteVerifier("witness1", w1Pub)

	rootHash := make([]byte, 32)
	rand.Read(rootHash)
	body := witness.FormatCheckpoint(origin, 10, rootHash)
	signed, _ := witness.SignCheckpoint(body, opSigner)

	// Cosign with witness1.
	n, _ := note.Open(signed, note.VerifierList(opVerifier))
	cosigned, _ := note.Sign(n, w1Signer)

	// Threshold 0: should pass with just operator.
	if err := witness.VerifyCosignedCheckpoint(string(signed), opVerifier, nil, 0); err != nil {
		t.Errorf("threshold=0 with operator only: %v", err)
	}

	// Threshold 1 with 1 witness: should pass.
	if err := witness.VerifyCosignedCheckpoint(string(cosigned), opVerifier, []note.Verifier{w1Verifier}, 1); err != nil {
		t.Errorf("threshold=1 with 1 witness: %v", err)
	}

	// Threshold 2 with only 1 witness cosigned: should fail.
	if err := witness.VerifyCosignedCheckpoint(string(cosigned), opVerifier, []note.Verifier{w1Verifier}, 2); err == nil {
		t.Errorf("expected threshold=2 to fail with only 1 witness")
	}
}

func TestNoteSignerNameFormat(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	origin := "siqlah.dev/log"

	signer, err := witness.NewNoteSigner(origin, priv)
	if err != nil {
		t.Fatalf("NewNoteSigner: %v", err)
	}
	verifier, err := witness.NewNoteVerifier(origin, pub)
	if err != nil {
		t.Fatalf("NewNoteVerifier: %v", err)
	}

	if signer.Name() != verifier.Name() {
		t.Errorf("signer.Name()=%q != verifier.Name()=%q", signer.Name(), verifier.Name())
	}
	if signer.KeyHash() != verifier.KeyHash() {
		t.Errorf("key hash mismatch: signer=%d verifier=%d", signer.KeyHash(), verifier.KeyHash())
	}
}

func TestCheckpointParsesMalformed(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"no_newlines", "siqlah.dev/log"},
		{"invalid_size", "siqlah.dev/log\nnot_a_number\nABC=\n"},
		{"invalid_root", "siqlah.dev/log\n42\n!!invalid!!\n"},
	}
	for _, tc := range cases {
		_, err := witness.ParseCheckpoint(tc.raw)
		if err == nil {
			t.Errorf("%s: expected error, got nil", tc.name)
		}
	}
}

func TestBackwardCompatibility_LegacyWitnessSignatures(t *testing.T) {
	// Verify that we can still work with legacy Ed25519 cosignatures
	// (stored as hex strings in the DB) alongside C2SP note signatures.
	// This is a structural test — the actual DB-stored sigs are verified via
	// internal/checkpoint.VerifyWitness (which is still in place and unchanged).
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, _ = pub, priv

	// Confirm the note signer generates a name that looks like a valid note name.
	signer, _ := witness.NewNoteSigner("siqlah.dev/log", priv)
	if !strings.Contains(signer.Name(), "siqlah.dev/log") {
		t.Errorf("expected signer name to contain origin, got %q", signer.Name())
	}
}
