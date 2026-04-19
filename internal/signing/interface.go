package signing

// SignatureBundle holds the result of a signing operation.
type SignatureBundle struct {
	SignatureHex   string
	CertificatePEM string
	RekorLogIndex  int64
	SignerIdentity string
	SignerType     string
	// BundleJSON holds the full serialized sigstore bundle for Fulcio-backed signatures.
	// Empty for Ed25519 signatures.
	BundleJSON string
}

// Signer signs arbitrary payloads and returns a SignatureBundle.
type Signer interface {
	Sign(payload []byte) (*SignatureBundle, error)
	Identity() string
	Type() string
}

// Verifier verifies a payload against a SignatureBundle.
type Verifier interface {
	Verify(payload []byte, bundle *SignatureBundle) error
}
