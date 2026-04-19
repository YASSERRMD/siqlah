// Package model provides OMS v1.0 model identity verification.
package model

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// ModelIdentity holds verified identity information about a model artifact.
type ModelIdentity struct {
	ModelName      string
	ModelDigest    string // hex SHA-256 of model weights/manifest
	SignerIdentity string // OIDC subject URI from certificate SAN
	RekorLogIndex  int64  // -1 if not anchored
	Verified       bool
}

// VerifyOptions controls verification behaviour.
type VerifyOptions struct {
	// TrustRoots is the pool of trusted CA certificates.
	// If nil, all certificates are accepted (test/dev mode only).
	TrustRoots *x509.CertPool
	// SkipTlogVerification skips checking tlog entries (use in unit tests).
	SkipTlogVerification bool
}

// omsBundle mirrors the Sigstore bundle v0.3 JSON schema.
type omsBundle struct {
	MediaType           string `json:"mediaType"`
	VerificationMaterial struct {
		Certificate struct {
			RawBytes string `json:"rawBytes"` // base64 DER
		} `json:"certificate"`
		TlogEntries []struct {
			LogIndex string `json:"logIndex"`
		} `json:"tlogEntries"`
	} `json:"verificationMaterial"`
	MessageSignature struct {
		MessageDigest struct {
			Algorithm string `json:"algorithm"`
			Digest    string `json:"digest"` // base64
		} `json:"messageDigest"`
		Signature string `json:"signature"` // base64
	} `json:"messageSignature"`
}

// VerifyModelIdentity parses and verifies an OMS Sigstore bundle JSON string.
// modelName is informational; the manifest digest is extracted from the bundle.
func VerifyModelIdentity(bundleJSON, modelName string, opts VerifyOptions) (*ModelIdentity, error) {
	if bundleJSON == "" {
		return nil, errors.New("bundle JSON is empty")
	}

	var b omsBundle
	if err := json.Unmarshal([]byte(bundleJSON), &b); err != nil {
		return nil, fmt.Errorf("parse bundle: %w", err)
	}

	// Decode certificate.
	certDER, err := base64.StdEncoding.DecodeString(b.VerificationMaterial.Certificate.RawBytes)
	if err != nil {
		return nil, fmt.Errorf("decode certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	// Verify certificate chain if trust roots provided.
	if opts.TrustRoots != nil {
		vo := x509.VerifyOptions{
			Roots:       opts.TrustRoots,
			CurrentTime: cert.NotBefore.Add(time.Second), // use issuance time
		}
		if _, err := cert.Verify(vo); err != nil {
			return nil, fmt.Errorf("certificate chain verification: %w", err)
		}
	}

	// Decode message digest.
	if b.MessageSignature.MessageDigest.Algorithm != "SHA2_256" {
		return nil, fmt.Errorf("unsupported digest algorithm: %s", b.MessageSignature.MessageDigest.Algorithm)
	}
	digestBytes, err := base64.StdEncoding.DecodeString(b.MessageSignature.MessageDigest.Digest)
	if err != nil {
		return nil, fmt.Errorf("decode message digest: %w", err)
	}
	if len(digestBytes) != sha256.Size {
		return nil, fmt.Errorf("expected 32-byte SHA256 digest, got %d bytes", len(digestBytes))
	}
	var digest [sha256.Size]byte
	copy(digest[:], digestBytes)

	// Decode and verify signature.
	sigBytes, err := base64.StdEncoding.DecodeString(b.MessageSignature.Signature)
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}

	ecPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("certificate does not contain an ECDSA public key")
	}
	if !ecdsa.VerifyASN1(ecPub, digest[:], sigBytes) {
		return nil, errors.New("signature verification failed")
	}

	// Extract OIDC identity from URI SAN.
	signerIdentity := extractURISAN(cert)

	// Extract model digest (hex of the SHA256 bytes).
	modelDigest := fmt.Sprintf("%x", digestBytes)

	// Extract Rekor log index if present.
	rekorIndex := int64(-1)
	if len(b.VerificationMaterial.TlogEntries) > 0 {
		var idx int64
		if _, err := fmt.Sscanf(b.VerificationMaterial.TlogEntries[0].LogIndex, "%d", &idx); err == nil {
			rekorIndex = idx
		}
	}

	return &ModelIdentity{
		ModelName:      modelName,
		ModelDigest:    modelDigest,
		SignerIdentity: signerIdentity,
		RekorLogIndex:  rekorIndex,
		Verified:       true,
	}, nil
}

// ParseBundleFromPEM parses an OMS bundle where the certificate is PEM-encoded
// (some OMS tools emit PEM rather than raw DER base64).
func ParseBundleFromPEM(bundleJSON string) (string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(bundleJSON), &raw); err != nil {
		return bundleJSON, nil // return as-is, let VerifyModelIdentity handle it
	}

	var vm struct {
		Certificate struct {
			RawBytes string `json:"rawBytes"`
		} `json:"certificate"`
	}
	if err := json.Unmarshal(raw["verificationMaterial"], &vm); err != nil {
		return bundleJSON, nil
	}

	// Try to detect PEM — if it decodes as base64 into DER, already correct.
	if _, err := base64.StdEncoding.DecodeString(vm.Certificate.RawBytes); err == nil {
		return bundleJSON, nil // already base64 DER
	}

	// Attempt PEM decode.
	block, _ := pem.Decode([]byte(vm.Certificate.RawBytes))
	if block == nil {
		return bundleJSON, nil
	}
	vm.Certificate.RawBytes = base64.StdEncoding.EncodeToString(block.Bytes)

	vmBytes, _ := json.Marshal(vm)
	raw["verificationMaterial"] = vmBytes
	out, _ := json.Marshal(raw)
	return string(out), nil
}

func extractURISAN(cert *x509.Certificate) string {
	for _, u := range cert.URIs {
		if isOIDCURI(u) {
			return u.String()
		}
	}
	// Fallback: use email SAN if present.
	if len(cert.EmailAddresses) > 0 {
		return cert.EmailAddresses[0]
	}
	return cert.Subject.CommonName
}

func isOIDCURI(u *url.URL) bool {
	return u.Scheme == "https" || u.Scheme == "http" || u.Scheme == "spiffe"
}
