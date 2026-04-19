package witness

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"golang.org/x/mod/sumdb/note"
)

// NewNoteSigner creates a note.Signer from an Ed25519 private key and log name.
// The key format follows the golang.org/x/mod/sumdb/note private key spec.
func NewNoteSigner(logName string, priv ed25519.PrivateKey) (note.Signer, error) {
	pub := priv.Public().(ed25519.PublicKey)
	pubBytes := append([]byte{noteAlgEd25519}, pub...)
	h := noteKeyHash(logName, pubBytes)
	privBytes := append([]byte{noteAlgEd25519}, priv.Seed()...)
	skey := fmt.Sprintf("PRIVATE+KEY+%s+%08x+%s",
		logName, h, base64.StdEncoding.EncodeToString(privBytes))
	return note.NewSigner(skey)
}

// NewNoteVerifier creates a note.Verifier from an Ed25519 public key and log name.
func NewNoteVerifier(logName string, pub ed25519.PublicKey) (note.Verifier, error) {
	vkey, err := note.NewEd25519VerifierKey(logName, pub)
	if err != nil {
		return nil, err
	}
	return note.NewVerifier(vkey)
}

const noteAlgEd25519 = 1

func noteKeyHash(name string, key []byte) uint32 {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte("\n"))
	h.Write(key)
	sum := h.Sum(nil)
	return binary.BigEndian.Uint32(sum[:4])
}
