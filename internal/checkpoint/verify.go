package checkpoint

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/yasserrmd/siqlah/internal/store"
)

// VerifyOperatorSignature verifies the operator's Ed25519 signature on a checkpoint.
func VerifyOperatorSignature(cp store.Checkpoint, pubKey ed25519.PublicKey) error {
	if cp.OperatorSigHex == "" {
		return errors.New("checkpoint has no operator signature")
	}
	sig, err := hex.DecodeString(cp.OperatorSigHex)
	if err != nil {
		return fmt.Errorf("decode operator sig: %w", err)
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
	if !ed25519.Verify(pubKey, pb, sig) {
		return errors.New("operator signature mismatch")
	}
	return nil
}

// VerifyChainConsistency checks that prev.RootHex matches curr.PreviousRootHex,
// ensuring the checkpoint chain is append-only.
func VerifyChainConsistency(prev, curr store.Checkpoint) error {
	if curr.PreviousRootHex == "" && prev.RootHex == "" {
		return nil
	}
	if curr.PreviousRootHex != prev.RootHex {
		return fmt.Errorf("chain broken: checkpoint %d root %s != checkpoint %d previous_root %s",
			prev.ID, prev.RootHex, curr.ID, curr.PreviousRootHex)
	}
	return nil
}
