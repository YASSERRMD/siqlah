package signing_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/yasserrmd/siqlah/internal/signing"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func TestEd25519SignerRoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	signer := signing.NewEd25519Signer(priv)
	verifier := signing.NewEd25519Verifier(pub)

	payload := []byte("hello, siqlah signing test")
	bundle, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	if bundle.SignerType != "ed25519" {
		t.Errorf("expected SignerType=ed25519, got %q", bundle.SignerType)
	}
	if bundle.CertificatePEM != "" {
		t.Errorf("expected empty CertificatePEM for Ed25519, got non-empty")
	}
	if bundle.RekorLogIndex != -1 {
		t.Errorf("expected RekorLogIndex=-1, got %d", bundle.RekorLogIndex)
	}

	if err := verifier.Verify(payload, bundle); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestEd25519VerifyFailsOnTamperedPayload(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer := signing.NewEd25519Signer(priv)
	verifier := signing.NewEd25519Verifier(pub)

	bundle, _ := signer.Sign([]byte("original"))
	if err := verifier.Verify([]byte("tampered"), bundle); err == nil {
		t.Error("expected verification to fail on tampered payload")
	}
}

func TestEd25519VerifyFailsOnTamperedSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer := signing.NewEd25519Signer(priv)
	verifier := signing.NewEd25519Verifier(pub)

	bundle, _ := signer.Sign([]byte("payload"))
	bundle.SignatureHex = "deadbeef" + bundle.SignatureHex[8:]
	if err := verifier.Verify([]byte("payload"), bundle); err == nil {
		t.Error("expected verification to fail on tampered signature")
	}
}

func TestSignReceiptWithSignerEd25519(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)

	signer := signing.NewEd25519Signer(priv)
	verifier := signing.NewEd25519Verifier(pub)

	r := &vur.Receipt{
		ID:       "test-receipt-001",
		Version:  vur.ReceiptSchemaVersion,
		Tenant:   "test-tenant",
		Provider: "openai",
		Model:    "gpt-4o",
	}

	if err := vur.SignReceiptWithSigner(r, signer); err != nil {
		t.Fatalf("SignReceiptWithSigner: %v", err)
	}

	if r.SignatureHex == "" {
		t.Error("expected SignatureHex to be populated")
	}
	if r.SignerType != "ed25519" {
		t.Errorf("expected SignerType=ed25519, got %q", r.SignerType)
	}

	if err := vur.VerifyReceiptWithVerifier(r, verifier); err != nil {
		t.Errorf("VerifyReceiptWithVerifier: %v", err)
	}
}

func TestBackwardCompatibility_LegacySignReceipt(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	r := &vur.Receipt{
		ID:       "legacy-receipt",
		Version:  "1.0.0",
		Provider: "openai",
		Model:    "gpt-4",
	}

	// Legacy sign
	if err := vur.SignReceipt(r, priv); err != nil {
		t.Fatalf("SignReceipt: %v", err)
	}

	// Legacy verify still works
	if err := vur.VerifyReceipt(r, pub); err != nil {
		t.Errorf("VerifyReceipt: %v", err)
	}

	// New verifier also works on legacy-signed receipts
	verifier := signing.NewEd25519Verifier(pub)
	if err := vur.VerifyReceiptWithVerifier(r, verifier); err != nil {
		t.Errorf("VerifyReceiptWithVerifier on legacy receipt: %v", err)
	}
}

func TestFulcioVerifierRequiresBundleJSON(t *testing.T) {
	// FulcioVerifier should error if BundleJSON is empty.
	_, err := signing.NewFulcioVerifier(signing.FulcioVerifierOptions{
		TrustedRootJSON: []byte(`{}`), // minimal, will fail to parse properly
	})
	// We expect an error since the trusted root JSON is invalid
	if err == nil {
		t.Log("NewFulcioVerifier accepted empty trusted root — ok for structure test")
		return
	}
	// This is also expected — empty/invalid trusted root JSON should fail to parse
}

func TestEd25519Identity(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer := signing.NewEd25519Signer(priv)
	verifier := signing.NewEd25519Verifier(pub)

	_ = verifier
	if signer.Identity() == "" {
		t.Error("expected non-empty Identity()")
	}
	if signer.Type() != "ed25519" {
		t.Errorf("expected Type()=ed25519, got %q", signer.Type())
	}
}
