package witness

import (
	"errors"
	"fmt"

	"golang.org/x/mod/sumdb/note"
)

// VerifyCosignedCheckpoint verifies a cosigned checkpoint note.
// It requires:
//   - The log operator's signature (verified via operatorVerifier).
//   - At least threshold valid witness cosignatures from requiredWitnesses.
//
// Returns nil only if all checks pass.
func VerifyCosignedCheckpoint(raw string, operatorVerifier note.Verifier, requiredWitnesses []note.Verifier, threshold int) error {
	if threshold < 0 {
		return errors.New("threshold must be >= 0")
	}

	// First verify the operator signature.
	allVerifiers := make([]note.Verifier, 0, len(requiredWitnesses)+1)
	allVerifiers = append(allVerifiers, operatorVerifier)
	allVerifiers = append(allVerifiers, requiredWitnesses...)

	n, err := note.Open([]byte(raw), note.VerifierList(allVerifiers...))
	if err != nil {
		// UnverifiedNoteError means no known verifier could verify — could be threshold issue.
		var uvErr *note.UnverifiedNoteError
		if errors.As(err, &uvErr) {
			n = uvErr.Note
		} else {
			return fmt.Errorf("open cosigned checkpoint: %w", err)
		}
	}

	// Check operator signature is verified.
	operatorVerified := false
	for _, sig := range n.Sigs {
		if sig.Name == operatorVerifier.Name() {
			operatorVerified = true
			break
		}
	}
	if !operatorVerified {
		return fmt.Errorf("operator signature by %q not verified", operatorVerifier.Name())
	}

	// Count verified witness cosignatures.
	witnessNames := map[string]bool{}
	for _, v := range requiredWitnesses {
		witnessNames[v.Name()] = true
	}

	verified := 0
	for _, sig := range n.Sigs {
		if witnessNames[sig.Name] {
			verified++
		}
	}

	if verified < threshold {
		return fmt.Errorf("insufficient witness signatures: got %d, need %d", verified, threshold)
	}
	return nil
}
