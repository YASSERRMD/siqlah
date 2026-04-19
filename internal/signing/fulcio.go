package signing

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	sigstorebundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/sign"
	"google.golang.org/protobuf/encoding/protojson"
)

// FulcioOptions configures Fulcio keyless signing.
type FulcioOptions struct {
	// FulcioURL is the Fulcio CA endpoint.
	FulcioURL string
	// RekorURL is the Rekor transparency log endpoint. Empty disables Rekor logging.
	RekorURL string
	// OIDCToken is the OIDC JWT presented to Fulcio to obtain a certificate.
	OIDCToken string
	// Timeout for CA and log requests (default: 30s).
	Timeout time.Duration
}

// FulcioSigner implements Signer using Sigstore Fulcio keyless signing.
// Each call to Sign generates a fresh ephemeral key and Fulcio certificate.
type FulcioSigner struct {
	opts FulcioOptions
}

// NewFulcioSigner creates a FulcioSigner.
func NewFulcioSigner(opts FulcioOptions) *FulcioSigner {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.FulcioURL == "" {
		opts.FulcioURL = "https://fulcio.sigstore.dev"
	}
	return &FulcioSigner{opts: opts}
}

func (s *FulcioSigner) Sign(payload []byte) (*SignatureBundle, error) {
	ctx := context.Background()

	keypair, err := sign.NewEphemeralKeypair(nil)
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral keypair: %w", err)
	}

	fulcioProvider := sign.NewFulcio(&sign.FulcioOptions{
		BaseURL: s.opts.FulcioURL,
		Timeout: s.opts.Timeout,
		Retries: 1,
	})

	bundleOpts := sign.BundleOptions{
		Context:             ctx,
		CertificateProvider: fulcioProvider,
		CertificateProviderOptions: &sign.CertificateProviderOptions{
			IDToken: s.opts.OIDCToken,
		},
	}

	if s.opts.RekorURL != "" {
		bundleOpts.TransparencyLogs = []sign.Transparency{
			sign.NewRekor(&sign.RekorOptions{
				BaseURL: s.opts.RekorURL,
				Timeout: s.opts.Timeout,
				Retries: 1,
			}),
		}
	}

	pb, err := sign.Bundle(&sign.PlainData{Data: payload}, keypair, bundleOpts)
	if err != nil {
		return nil, fmt.Errorf("sigstore bundle signing: %w", err)
	}

	bundleJSON, err := protojson.Marshal(pb)
	if err != nil {
		return nil, fmt.Errorf("marshal bundle: %w", err)
	}

	certPEM, identity, err := extractCertInfo(pb)
	if err != nil {
		return nil, err
	}

	sigHex := hex.EncodeToString(pb.GetMessageSignature().GetSignature())

	rekorIdx := int64(-1)
	if entries := pb.GetVerificationMaterial().GetTlogEntries(); len(entries) > 0 {
		rekorIdx = entries[0].GetLogIndex()
	}

	return &SignatureBundle{
		SignatureHex:   sigHex,
		CertificatePEM: certPEM,
		RekorLogIndex:  rekorIdx,
		SignerIdentity: identity,
		SignerType:     s.Type(),
		BundleJSON:     string(bundleJSON),
	}, nil
}

func (s *FulcioSigner) Identity() string {
	return "fulcio:" + s.opts.FulcioURL
}

func (s *FulcioSigner) Type() string {
	return "fulcio"
}

// extractCertInfo pulls the certificate PEM and OIDC subject URI from a Fulcio bundle.
func extractCertInfo(pb *protobundle.Bundle) (certPEM, identity string, err error) {
	rawCert := pb.GetVerificationMaterial().GetCertificate().GetRawBytes()
	if len(rawCert) == 0 {
		return "", "", fmt.Errorf("bundle has no certificate")
	}

	certPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCert,
	}))

	cert, err := x509.ParseCertificate(rawCert)
	if err != nil {
		return certPEM, "", fmt.Errorf("parse certificate: %w", err)
	}

	// Fulcio encodes the OIDC subject in the URI SAN.
	for _, uri := range cert.URIs {
		if uri != nil {
			identity = uri.String()
			break
		}
	}
	if identity == "" && len(cert.EmailAddresses) > 0 {
		identity = cert.EmailAddresses[0]
	}

	return certPEM, identity, nil
}

// ParseFulcioBundle deserializes a sigstore bundle JSON for verification.
func ParseFulcioBundle(bundleJSON string) (*sigstorebundle.Bundle, error) {
	var pb protobundle.Bundle
	if err := protojson.Unmarshal([]byte(bundleJSON), &pb); err != nil {
		return nil, fmt.Errorf("unmarshal bundle JSON: %w", err)
	}
	return sigstorebundle.NewBundle(&pb)
}
