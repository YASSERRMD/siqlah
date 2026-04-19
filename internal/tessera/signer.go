package tessera

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/sumdb/note"
)

const noteAlgEd25519 = 1

// NewNoteSigner wraps an existing Ed25519 private key as a note.Signer for Tessera.
// logName is the human-readable log origin (e.g. "siqlah.dev/log").
func NewNoteSigner(logName string, privKey ed25519.PrivateKey) (note.Signer, error) {
	pub := privKey.Public().(ed25519.PublicKey)
	pubKeyBytes := append([]byte{noteAlgEd25519}, pub...)
	h := noteKeyHash(logName, pubKeyBytes)

	privKeyBytes := append([]byte{noteAlgEd25519}, privKey.Seed()...)
	skey := fmt.Sprintf("PRIVATE+KEY+%s+%08x+%s",
		logName, h, base64.StdEncoding.EncodeToString(privKeyBytes))

	signer, err := note.NewSigner(skey)
	if err != nil {
		return nil, fmt.Errorf("create note signer: %w", err)
	}
	return signer, nil
}

// NewNoteVerifier wraps an Ed25519 public key as a note.Verifier for checkpoint verification.
func NewNoteVerifier(logName string, pub ed25519.PublicKey) (note.Verifier, error) {
	vkey, err := note.NewEd25519VerifierKey(logName, pub)
	if err != nil {
		return nil, fmt.Errorf("create note verifier key: %w", err)
	}
	verifier, err := note.NewVerifier(vkey)
	if err != nil {
		return nil, fmt.Errorf("create note verifier: %w", err)
	}
	return verifier, nil
}

// noteKeyHash computes the note key hash: SHA-256(name + "\n" + keyBytes), returned as uint32 big-endian.
func noteKeyHash(name string, keyBytes []byte) uint32 {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte("\n"))
	h.Write(keyBytes)
	sum := h.Sum(nil)
	return binary.BigEndian.Uint32(sum)
}

// ParseCheckpointSize parses the tree size from a raw C2SP signed note checkpoint.
// Checkpoint format:
//
//	<origin>
//	<tree_size>
//	<root_hash_base64>
//
//	— <signer> <keyhint> <sig>
func ParseCheckpointSize(raw []byte) (uint64, error) {
	lines := strings.SplitN(string(raw), "\n", 4)
	if len(lines) < 3 {
		return 0, fmt.Errorf("malformed checkpoint: too few lines")
	}
	size, err := strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse checkpoint size: %w", err)
	}
	return size, nil
}

// ParseCheckpointRoot parses the Merkle root from a raw C2SP signed note checkpoint.
func ParseCheckpointRoot(raw []byte) ([]byte, error) {
	lines := strings.SplitN(string(raw), "\n", 4)
	if len(lines) < 3 {
		return nil, fmt.Errorf("malformed checkpoint: too few lines")
	}
	rootB64 := strings.TrimSpace(lines[2])
	root, err := base64.StdEncoding.DecodeString(rootB64)
	if err != nil {
		return nil, fmt.Errorf("decode checkpoint root: %w", err)
	}
	return root, nil
}
