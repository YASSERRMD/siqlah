package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/store"
)

func TestKeygenProducesValidKeys(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "witness.key")
	if err := runKeygen([]string{"--out", keyFile}); err != nil {
		t.Fatalf("keygen: %v", err)
	}

	priv, pub, err := loadPrivKey(keyFile)
	if err != nil {
		t.Fatalf("loadPrivKey: %v", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("expected %d-byte private key", ed25519.PrivateKeySize)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Errorf("expected %d-byte public key", ed25519.PublicKeySize)
	}

	msg := []byte("test message")
	sig := ed25519.Sign(priv, msg)
	if !ed25519.Verify(pub, msg, sig) {
		t.Error("generated key pair does not produce valid signatures")
	}
}

func TestKeygenStdout(t *testing.T) {
	if err := runKeygen([]string{}); err != nil {
		t.Fatalf("keygen to stdout: %v", err)
	}
}

func TestHexToEd25519Pub(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	got, err := hexToEd25519Pub(hex.EncodeToString(pub))
	if err != nil {
		t.Fatalf("hexToEd25519Pub: %v", err)
	}
	if !pub.Equal(got) {
		t.Error("public key round-trip mismatch")
	}
}

func TestLoadPrivKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(t.TempDir(), "*.key")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(hex.EncodeToString(priv))
	f.Close()

	loaded, _, err := loadPrivKey(f.Name())
	if err != nil {
		t.Fatalf("loadPrivKey: %v", err)
	}
	if !priv.Equal(loaded) {
		t.Error("private key round-trip mismatch")
	}
}

func TestCosignWithMockServer(t *testing.T) {
	opPub, opPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, witnessPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	cp := buildSignedCheckpoint(t, opPriv)

	var submittedWID, submittedSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/checkpoints/1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cp)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/checkpoints/1/witness":
			var req map[string]string
			json.NewDecoder(r.Body).Decode(&req)
			submittedWID = req["witness_id"]
			submittedSig = req["sig_hex"]
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	keyFile := filepath.Join(t.TempDir(), "witness.key")
	os.WriteFile(keyFile, []byte(hex.EncodeToString(witnessPriv)), 0600)

	if err := runCosign([]string{
		"--ledger", srv.URL,
		"--cp", "1",
		"--key", keyFile,
		"--op-pub", hex.EncodeToString(opPub),
		"--witness-id", "test-witness",
	}); err != nil {
		t.Fatalf("cosign: %v", err)
	}

	if submittedWID != "test-witness" {
		t.Errorf("expected witness_id=test-witness, got %q", submittedWID)
	}
	if submittedSig == "" {
		t.Error("expected non-empty sig_hex to be submitted")
	}
}

func TestVerifyWithMockServer(t *testing.T) {
	opPub, opPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cp := buildSignedCheckpoint(t, opPriv)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/checkpoints/1":
			json.NewEncoder(w).Encode(cp)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/checkpoints/1/verify":
			json.NewEncoder(w).Encode(map[string]any{
				"operator_valid": true,
				"witnesses":      map[string]string{"w1": "aabbcc"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	if err := runVerify([]string{
		"--ledger", srv.URL,
		"--cp", "1",
		"--op-pub", hex.EncodeToString(opPub),
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

// buildSignedCheckpoint creates a store.Checkpoint signed by opPriv via the
// checkpoint store+builder pipeline, using an in-memory SQLite database.
func buildSignedCheckpoint(t *testing.T, opPriv ed25519.PrivateKey) store.Checkpoint {
	t.Helper()

	// We can't use the full Builder without a real receipt, so construct the
	// checkpoint and sign the payload directly using the exported checkpoint types.
	// Build a SignedPayload using the same logic as checkpoint.Builder.
	now := time.Now().UTC().Truncate(time.Second)
	cp := store.Checkpoint{
		ID:         1,
		BatchStart: 1,
		BatchEnd:   1,
		TreeSize:   1,
		RootHex:    "0000000000000000000000000000000000000000000000000000000000000000",
		IssuedAt:   now,
	}

	sp := &checkpoint.SignedPayload{
		BatchStart:      cp.BatchStart,
		BatchEnd:        cp.BatchEnd,
		TreeSize:        cp.TreeSize,
		RootHex:         cp.RootHex,
		PreviousRootHex: cp.PreviousRootHex,
		IssuedAt:        now.Format("2006-01-02T15:04:05.999999999Z"),
	}
	pb, err := sp.Bytes()
	if err != nil {
		t.Fatalf("payload bytes: %v", err)
	}
	sig := ed25519.Sign(opPriv, pb)
	cp.OperatorSigHex = hex.EncodeToString(sig)

	// Verify the signature matches what checkpoint.VerifyOperatorSignature expects.
	if err := checkpoint.VerifyOperatorSignature(cp, opPriv.Public().(ed25519.PublicKey)); err != nil {
		t.Fatalf("self-check: built checkpoint has invalid operator sig: %v", err)
	}
	return cp
}
