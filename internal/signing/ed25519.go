package signing

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// Ed25519Signer implements Signer using an Ed25519 private key.
type Ed25519Signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

// NewEd25519Signer creates a Signer from an Ed25519 private key.
func NewEd25519Signer(priv ed25519.PrivateKey) *Ed25519Signer {
	return &Ed25519Signer{
		priv: priv,
		pub:  priv.Public().(ed25519.PublicKey),
	}
}

func (s *Ed25519Signer) Sign(payload []byte) (*SignatureBundle, error) {
	sig := ed25519.Sign(s.priv, payload)
	return &SignatureBundle{
		SignatureHex:   hex.EncodeToString(sig),
		CertificatePEM: "",
		RekorLogIndex:  -1,
		SignerIdentity: s.Identity(),
		SignerType:     s.Type(),
	}, nil
}

func (s *Ed25519Signer) Identity() string {
	return hex.EncodeToString(s.pub)
}

func (s *Ed25519Signer) Type() string {
	return "ed25519"
}

// Ed25519Verifier implements Verifier using an Ed25519 public key.
type Ed25519Verifier struct {
	pub ed25519.PublicKey
}

// NewEd25519Verifier creates a Verifier from an Ed25519 public key.
func NewEd25519Verifier(pub ed25519.PublicKey) *Ed25519Verifier {
	return &Ed25519Verifier{pub: pub}
}

func (v *Ed25519Verifier) Verify(payload []byte, bundle *SignatureBundle) error {
	sig, err := hex.DecodeString(bundle.SignatureHex)
	if err != nil {
		return fmt.Errorf("decode signature hex: %w", err)
	}
	if !ed25519.Verify(v.pub, payload, sig) {
		return fmt.Errorf("ed25519 signature verification failed")
	}
	return nil
}
