package model

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/url"
	"testing"
	"time"
)

// buildTestBundle creates a self-signed OMS-style bundle for unit tests.
// The certificate has a URI SAN of signerURI.
func buildTestBundle(t *testing.T, payload []byte, signerURI string) string {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	u, _ := url.Parse(signerURI)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	digest := sha256.Sum256(payload)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	bundle := map[string]any{
		"mediaType": "application/vnd.dev.sigstore.bundle.v0.3+json",
		"verificationMaterial": map[string]any{
			"certificate": map[string]string{
				"rawBytes": base64.StdEncoding.EncodeToString(certDER),
			},
			"tlogEntries": []map[string]string{
				{"logIndex": "42"},
			},
		},
		"messageSignature": map[string]any{
			"messageDigest": map[string]string{
				"algorithm": "SHA2_256",
				"digest":    base64.StdEncoding.EncodeToString(digest[:]),
			},
			"signature": base64.StdEncoding.EncodeToString(sig),
		},
	}
	b, _ := json.Marshal(bundle)
	return string(b)
}

func TestVerifyModelIdentity_Valid(t *testing.T) {
	payload := []byte("model-manifest: llama-3.1-8b\nsha256: abc123")
	bundleJSON := buildTestBundle(t, payload, "https://huggingface.co/meta-llama")

	id, err := VerifyModelIdentity(bundleJSON, "meta-llama/Llama-3.1-8B", VerifyOptions{})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !id.Verified {
		t.Error("expected Verified=true")
	}
	if id.SignerIdentity != "https://huggingface.co/meta-llama" {
		t.Errorf("signer identity: got %q", id.SignerIdentity)
	}
	if id.ModelName != "meta-llama/Llama-3.1-8B" {
		t.Errorf("model name: got %q", id.ModelName)
	}
	if id.RekorLogIndex != 42 {
		t.Errorf("rekor log index: got %d", id.RekorLogIndex)
	}
	if id.ModelDigest == "" {
		t.Error("expected non-empty model digest")
	}
}

func TestVerifyModelIdentity_TamperedSignature(t *testing.T) {
	payload := []byte("model-manifest: llama-3.1-8b")
	bundleJSON := buildTestBundle(t, payload, "https://example.com")

	// Tamper: flip a bit in the signature.
	var raw map[string]json.RawMessage
	json.Unmarshal([]byte(bundleJSON), &raw)
	var ms struct {
		MessageDigest struct {
			Algorithm string `json:"algorithm"`
			Digest    string `json:"digest"`
		} `json:"messageDigest"`
		Signature string `json:"signature"`
	}
	json.Unmarshal(raw["messageSignature"], &ms)
	sigBytes, _ := base64.StdEncoding.DecodeString(ms.Signature)

	// Corrupt the DER-encoded signature: flip a byte in the middle
	var seq asn1.RawValue
	asn1.Unmarshal(sigBytes, &seq)
	if len(seq.Bytes) > 4 {
		seq.Bytes[4] ^= 0xff
	}
	tampered, _ := asn1.Marshal(seq)
	ms.Signature = base64.StdEncoding.EncodeToString(tampered)
	msBytes, _ := json.Marshal(ms)
	raw["messageSignature"] = msBytes
	tampered2, _ := json.Marshal(raw)

	_, err := VerifyModelIdentity(string(tampered2), "test-model", VerifyOptions{})
	if err == nil {
		t.Error("expected error for tampered signature, got nil")
	}
}

func TestVerifyModelIdentity_EmptyBundle(t *testing.T) {
	_, err := VerifyModelIdentity("", "test-model", VerifyOptions{})
	if err == nil {
		t.Error("expected error for empty bundle")
	}
}

func TestVerifyModelIdentity_MalformedBundle(t *testing.T) {
	_, err := VerifyModelIdentity(`{"not": "a bundle"}`, "test-model", VerifyOptions{})
	if err == nil {
		t.Error("expected error for malformed bundle")
	}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()

	id := ModelIdentity{
		ModelDigest:    "abc123",
		SignerIdentity: "https://example.com",
		Verified:       true,
	}
	if err := r.Register("my-model", id); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := r.Lookup("my-model")
	if err != nil || got == nil {
		t.Fatalf("lookup: err=%v got=%v", err, got)
	}
	if got.SignerIdentity != "https://example.com" {
		t.Errorf("signer identity: got %q", got.SignerIdentity)
	}
	if got.ModelName != "my-model" {
		t.Errorf("model name: got %q", got.ModelName)
	}
}

func TestRegistry_LookupByDigest(t *testing.T) {
	r := NewRegistry()
	r.Register("digest-model", ModelIdentity{ModelDigest: "deadbeef", Verified: true})

	got, err := r.LookupByDigest("deadbeef")
	if err != nil || got == nil {
		t.Fatalf("lookup by digest: err=%v got=%v", err, got)
	}
	if got.ModelName != "digest-model" {
		t.Errorf("model name: got %q", got.ModelName)
	}
}

func TestRegistry_WellKnownModels(t *testing.T) {
	r := NewRegistry()

	for _, name := range []string{"gpt-4o", "claude-sonnet-4-5", "meta-llama/Llama-3.1-8B"} {
		id, err := r.Lookup(name)
		if err != nil {
			t.Errorf("lookup %q: %v", name, err)
			continue
		}
		if id == nil {
			t.Errorf("expected well-known entry for %q", name)
			continue
		}
		if id.SignerIdentity == "" {
			t.Errorf("expected non-empty signer identity for %q", name)
		}
	}
}

func TestRegistry_UnknownModel(t *testing.T) {
	r := NewRegistry()
	id, err := r.Lookup("unknown-model-xyz")
	if err != nil {
		t.Fatalf("lookup unknown: %v", err)
	}
	if id != nil {
		t.Errorf("expected nil for unknown model, got %+v", id)
	}
}

func TestRegistry_EmptyName(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("", ModelIdentity{}); err == nil {
		t.Error("expected error for empty model name")
	}
}
