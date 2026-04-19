// DEPRECATED: The custom cosigning format in this file is superseded by the C2SP
// tlog-cosignature/v1 protocol implemented in internal/witness/. Existing witness
// signatures created via CoSign/VerifyWitness remain valid for legacy checkpoints.
// New cosignatures should use internal/witness.WitnessClient.VerifyAndCosign instead.
package checkpoint

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/yasserrmd/siqlah/internal/store"
)

// CoSign verifies the operator signature on cp, then cosigns the checkpoint payload
// with the witness private key. Returns the hex-encoded witness signature.
func CoSign(cp store.Checkpoint, operatorPub ed25519.PublicKey, witnessKey ed25519.PrivateKey) (string, error) {
	if err := VerifyOperatorSignature(cp, operatorPub); err != nil {
		return "", fmt.Errorf("cosign: operator verification failed: %w", err)
	}
	payload := &SignedPayload{
		BatchStart:      cp.BatchStart,
		BatchEnd:        cp.BatchEnd,
		TreeSize:        cp.TreeSize,
		RootHex:         cp.RootHex,
		PreviousRootHex: cp.PreviousRootHex,
		IssuedAt:        cp.IssuedAt.UTC().Format("2006-01-02T15:04:05.999999999Z"),
	}
	pb, err := payload.Bytes()
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(witnessKey, pb)
	return hex.EncodeToString(sig), nil
}

// VerifyWitness verifies a witness cosignature against the checkpoint payload.
func VerifyWitness(cp store.Checkpoint, witnessPub ed25519.PublicKey, sigHex string) error {
	if sigHex == "" {
		return errors.New("witness signature is empty")
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("decode witness sig: %w", err)
	}
	payload := &SignedPayload{
		BatchStart:      cp.BatchStart,
		BatchEnd:        cp.BatchEnd,
		TreeSize:        cp.TreeSize,
		RootHex:         cp.RootHex,
		PreviousRootHex: cp.PreviousRootHex,
		IssuedAt:        cp.IssuedAt.UTC().Format("2006-01-02T15:04:05.999999999Z"),
	}
	pb, err := payload.Bytes()
	if err != nil {
		return err
	}
	if !ed25519.Verify(witnessPub, pb, sig) {
		return errors.New("witness signature mismatch")
	}
	return nil
}
