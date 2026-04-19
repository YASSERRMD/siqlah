package vur

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/yasserrmd/siqlah/internal/signing"
)

// ReceiptSchemaVersion is the current schema version for new receipts.
const ReceiptSchemaVersion = "1.1.0"

// SignReceiptWithSigner signs r using the provided signing.Signer, populating
// SignatureHex, SignerIdentity, SignerType, CertificatePEM, and RekorLogIndex.
func SignReceiptWithSigner(r *Receipt, s signing.Signer) error {
	r.SignerType = s.Type()
	r.SignerIdentity = s.Identity()
	r.SignatureHex = ""
	r.CertificatePEM = ""
	r.RekorLogIndex = 0

	b, err := r.CanonicalBytes()
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	bundle, err := s.Sign(b)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	r.SignatureHex = bundle.SignatureHex
	r.CertificatePEM = bundle.CertificatePEM
	if bundle.RekorLogIndex >= 0 {
		r.RekorLogIndex = bundle.RekorLogIndex
	}
	return nil
}

// VerifyReceiptWithVerifier verifies r using the provided signing.Verifier.
func VerifyReceiptWithVerifier(r *Receipt, v signing.Verifier) error {
	sigHex := r.SignatureHex
	if sigHex == "" {
		return errors.New("verify: receipt has no signature")
	}

	r.SignatureHex = ""
	b, err := r.CanonicalBytes()
	r.SignatureHex = sigHex
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	bundle := &signing.SignatureBundle{
		SignatureHex:   sigHex,
		CertificatePEM: r.CertificatePEM,
		RekorLogIndex:  r.RekorLogIndex,
		SignerIdentity: r.SignerIdentity,
		SignerType:     r.SignerType,
	}

	return v.Verify(b, bundle)
}

// SignReceipt signs the canonical bytes of r with privateKey and stores the hex signature.
// Deprecated: use SignReceiptWithSigner with signing.NewEd25519Signer instead.
func SignReceipt(r *Receipt, privateKey ed25519.PrivateKey) error {
	if r.SignerType == "" {
		r.SignerType = "ed25519"
	}
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
// Deprecated: use VerifyReceiptWithVerifier with signing.NewEd25519Verifier instead.
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
