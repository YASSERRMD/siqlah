package vur

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
)

// SignReceipt signs the canonical bytes of r with privateKey and stores the hex signature.
// The SignatureHex field must be empty before signing (it is not included in canonical bytes).
func SignReceipt(r *Receipt, privateKey ed25519.PrivateKey) error {
	r.SignatureHex = ""
	b, err := r.CanonicalBytes()
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	sig := ed25519.Sign(privateKey, b)
	r.SignatureHex = hex.EncodeToString(sig)
	return nil
}

// VerifyReceipt verifies the Ed25519 signature stored in r.SignatureHex against
// the canonical bytes of r (with SignatureHex temporarily cleared).
func VerifyReceipt(r *Receipt, publicKey ed25519.PublicKey) error {
	sigHex := r.SignatureHex
	if sigHex == "" {
		return errors.New("verify: receipt has no signature")
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("verify: invalid signature hex: %w", err)
	}

	r.SignatureHex = ""
	b, err := r.CanonicalBytes()
	r.SignatureHex = sigHex
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if !ed25519.Verify(publicKey, b, sig) {
		return errors.New("verify: signature mismatch")
	}
	return nil
}
