package signing

import (
	"bytes"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// FulcioVerifierOptions configures how Fulcio-backed bundles are verified.
type FulcioVerifierOptions struct {
	// TrustedRootJSON is the JSON of the Sigstore trusted root (required).
	// Obtain from TUF or use root.GetTrustedRoot(tufClient).
	TrustedRootJSON []byte
	// RequireTlog requires a Rekor transparency log entry in the bundle.
	RequireTlog bool
	// ExpectedOIDCIssuer restricts which OIDC issuer is accepted (optional).
	ExpectedOIDCIssuer string
	// ExpectedSAN restricts which Subject Alternative Name is accepted (optional).
	ExpectedSAN string
}

// FulcioVerifier implements Verifier for Fulcio-signed bundles.
type FulcioVerifier struct {
	opts            FulcioVerifierOptions
	trustedMaterial root.TrustedMaterial
}

// NewFulcioVerifier creates a FulcioVerifier. TrustedRootJSON must be provided.
func NewFulcioVerifier(opts FulcioVerifierOptions) (*FulcioVerifier, error) {
	if len(opts.TrustedRootJSON) == 0 {
		return nil, fmt.Errorf("TrustedRootJSON is required; fetch via TUF or embed a static root JSON")
	}
	tm, err := root.NewTrustedRootFromJSON(opts.TrustedRootJSON)
	if err != nil {
		return nil, fmt.Errorf("load trusted root: %w", err)
	}
	return &FulcioVerifier{opts: opts, trustedMaterial: tm}, nil
}

func (v *FulcioVerifier) Verify(payload []byte, bundle *SignatureBundle) error {
	if bundle.SignerType != "fulcio" {
		return fmt.Errorf("expected fulcio bundle, got %q", bundle.SignerType)
	}
	if bundle.BundleJSON == "" {
		return fmt.Errorf("BundleJSON is required for Fulcio verification")
	}

	sb, err := ParseFulcioBundle(bundle.BundleJSON)
	if err != nil {
		return fmt.Errorf("parse bundle: %w", err)
	}

	verifierOpts := []verify.VerifierOption{verify.WithObserverTimestamps(1)}
	if v.opts.RequireTlog {
		verifierOpts = append(verifierOpts, verify.WithTransparencyLog(1))
	}

	verifier, err := verify.NewSignedEntityVerifier(v.trustedMaterial, verifierOpts...)
	if err != nil {
		return fmt.Errorf("create verifier: %w", err)
	}

	artifactPolicy := verify.WithArtifact(bytes.NewReader(payload))

	var policyOpts []verify.PolicyOption
	if v.opts.ExpectedOIDCIssuer != "" && v.opts.ExpectedSAN != "" {
		sanMatcher, err := verify.NewSANMatcher(v.opts.ExpectedSAN, "")
		if err != nil {
			return fmt.Errorf("build SAN matcher: %w", err)
		}
		issuerMatcher := verify.IssuerMatcher{Issuer: v.opts.ExpectedOIDCIssuer}
		certID, err := verify.NewCertificateIdentity(sanMatcher, issuerMatcher, certificate.Extensions{})
		if err != nil {
			return fmt.Errorf("build certificate identity: %w", err)
		}
		policyOpts = append(policyOpts, verify.WithCertificateIdentity(certID))
	}

	policy := verify.NewPolicy(artifactPolicy, policyOpts...)
	if _, err := verifier.Verify(sb, policy); err != nil {
		return fmt.Errorf("bundle verification failed: %w", err)
	}
	return nil
}
