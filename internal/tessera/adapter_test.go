package tessera_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/tessera"
)

func newTestLog(t *testing.T) (*tessera.TesseraLog, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "siqlah-tessera-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := tessera.NewNoteSigner("test.siqlah.dev/log", priv)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}

	ctx := context.Background()
	tlog, err := tessera.NewTesseraLog(ctx, dir, signer)
	if err != nil {
		t.Fatalf("create tessera log: %v", err)
	}
	t.Cleanup(func() { tlog.Close(context.Background()) })
	return tlog, dir
}

func TestTesseraAppend100(t *testing.T) {
	tlog, _ := newTestLog(t)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		data := []byte("receipt-entry-" + string(rune('0'+i%10)))
		idx, err := tlog.Append(ctx, data)
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if idx != uint64(i) {
			t.Errorf("append %d: got index %d, want %d", i, idx, i)
		}
	}

	// Wait for Tessera to integrate and publish a checkpoint.
	var size uint64
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		size, err = tlog.TreeSize(ctx)
		if err == nil && size >= 100 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if size < 100 {
		t.Fatalf("tree size %d after 10s, want >= 100", size)
	}
}

func TestTesseraInclusionProof(t *testing.T) {
	tlog, _ := newTestLog(t)
	ctx := context.Background()

	entries := [][]byte{
		[]byte("entry-alpha"),
		[]byte("entry-beta"),
		[]byte("entry-gamma"),
		[]byte("entry-delta"),
		[]byte("entry-epsilon"),
	}
	for _, e := range entries {
		if _, err := tlog.Append(ctx, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// Wait for checkpoint.
	var treeSize uint64
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		treeSize, err = tlog.TreeSize(ctx)
		if err == nil && treeSize >= uint64(len(entries)) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if treeSize < uint64(len(entries)) {
		t.Fatalf("tree size %d, want %d", treeSize, len(entries))
	}

	// Get inclusion proof for entry at index 2.
	proof, err := tlog.InclusionProof(ctx, 2, treeSize)
	if err != nil {
		t.Fatalf("InclusionProof: %v", err)
	}
	if len(proof) == 0 {
		t.Error("expected non-empty inclusion proof")
	}
}

func TestTesseraConsistencyProof(t *testing.T) {
	tlog, _ := newTestLog(t)
	ctx := context.Background()

	// Append 5 entries and record tree size.
	for i := 0; i < 5; i++ {
		if _, err := tlog.Append(ctx, []byte("entry-a")); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	var oldSize uint64
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		oldSize, err = tlog.TreeSize(ctx)
		if err == nil && oldSize >= 5 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if oldSize < 5 {
		t.Fatalf("old tree size %d, want >= 5", oldSize)
	}

	// Append 5 more.
	for i := 0; i < 5; i++ {
		if _, err := tlog.Append(ctx, []byte("entry-b")); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	var newSize uint64
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		newSize, err = tlog.TreeSize(ctx)
		if err == nil && newSize >= 10 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if newSize < 10 {
		t.Fatalf("new tree size %d, want >= 10", newSize)
	}

	proof, err := tlog.ConsistencyProof(ctx, oldSize, newSize)
	if err != nil {
		t.Fatalf("ConsistencyProof: %v", err)
	}
	if len(proof) == 0 {
		t.Error("expected non-empty consistency proof")
	}
}

func TestTesseraCheckpointSigningAndVerification(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)
	logName := "test.siqlah.dev/log"

	signer, err := tessera.NewNoteSigner(logName, priv)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	verifier, err := tessera.NewNoteVerifier(logName, pub)
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}

	if signer.Name() != logName {
		t.Errorf("signer name = %q, want %q", signer.Name(), logName)
	}
	if verifier.Name() != logName {
		t.Errorf("verifier name = %q, want %q", verifier.Name(), logName)
	}
}

func TestTesseraPersistenceAcrossRestart(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	dir, err := os.MkdirTemp("", "siqlah-tessera-persist-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	logName := "persist.test.siqlah.dev/log"
	ctx := context.Background()

	// First instance: append 3 entries.
	signer1, _ := tessera.NewNoteSigner(logName, priv)
	tlog1, err := tessera.NewTesseraLog(ctx, dir, signer1)
	if err != nil {
		t.Fatalf("create first tessera log: %v", err)
	}
	entry1 := []byte("persistent-entry-one")
	entry2 := []byte("persistent-entry-two")
	entry3 := []byte("persistent-entry-three")
	for _, e := range [][]byte{entry1, entry2, entry3} {
		if _, err := tlog1.Append(ctx, e); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	// Wait for checkpoint.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		size, err := tlog1.TreeSize(ctx)
		if err == nil && size >= 3 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	tlog1.Close(ctx)

	// Second instance: open the same dir and verify the checkpoint survives.
	signer2, _ := tessera.NewNoteSigner(logName, priv)
	tlog2, err := tessera.NewTesseraLog(ctx, dir, signer2)
	if err != nil {
		t.Fatalf("create second tessera log: %v", err)
	}
	defer tlog2.Close(ctx)

	// Wait briefly for the second instance to read the checkpoint.
	time.Sleep(500 * time.Millisecond)

	raw, err := tlog2.ReadCheckpoint(ctx)
	if err != nil {
		t.Fatalf("ReadCheckpoint on second instance: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty checkpoint after restart")
	}
	size, err := tessera.ParseCheckpointSize(raw)
	if err != nil {
		t.Fatalf("ParseCheckpointSize: %v", err)
	}
	if size < 3 {
		t.Errorf("tree size after restart = %d, want >= 3", size)
	}

	// Verify root is non-zero.
	root, err := tessera.ParseCheckpointRoot(raw)
	if err != nil {
		t.Fatalf("ParseCheckpointRoot: %v", err)
	}
	if bytes.Equal(root, make([]byte, len(root))) {
		t.Error("root hash is all zeros")
	}
}
