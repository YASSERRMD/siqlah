package witness

import (
	"fmt"
	"strings"

	"golang.org/x/mod/sumdb/note"
)

// MergeCosignatures combines an operator-signed checkpoint with additional cosigned
// checkpoints, returning a single note that contains all signatures.
// The operatorVerifier is used to filter out duplicate operator signatures.
func MergeCosignatures(operatorSigned []byte, cosignedNotes []string, operatorVerifier note.Verifier) ([]byte, error) {
	// Parse the operator's own note.
	baseNote, err := note.Open(operatorSigned, note.VerifierList(operatorVerifier))
	if err != nil {
		// Try with empty verifiers to get the unverified note
		unv, ue := note.Open(operatorSigned, note.VerifierList())
		if ue != nil {
			// UnverifiedNoteError still gives us the note
			if unvErr, ok := ue.(*note.UnverifiedNoteError); ok {
				baseNote = unvErr.Note
			} else {
				return nil, fmt.Errorf("parse operator note: %w", err)
			}
		} else {
			baseNote = unv
		}
	}

	// Collect all cosignature lines from witness notes.
	// Each cosigned note has the same body but different/additional signature lines.
	extraSigs := []note.Signature{}
	seen := map[string]bool{}
	for _, sig := range baseNote.Sigs {
		seen[sig.Base64] = true
	}
	for _, sig := range baseNote.UnverifiedSigs {
		seen[sig.Base64] = true
	}

	for _, cosigned := range cosignedNotes {
		// Extract the signature lines from each cosigned note.
		parts := strings.SplitN(cosigned, "\n\n", 2)
		if len(parts) < 2 {
			continue
		}
		for _, line := range strings.Split(parts[1], "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "— ") || line == "" {
				continue
			}
			// Parse "— <name> <b64hash+sig>"
			fields := strings.Fields(line[2:]) // strip "— "
			if len(fields) < 2 {
				continue
			}
			b64 := fields[len(fields)-1]
			if !seen[b64] {
				seen[b64] = true
				extraSigs = append(extraSigs, note.Signature{
					Name:   strings.Join(fields[:len(fields)-1], " "),
					Base64: b64,
				})
			}
		}
	}

	// Rebuild the merged note.
	mergedNote := &note.Note{
		Text:           baseNote.Text,
		Sigs:           baseNote.Sigs,
		UnverifiedSigs: append(baseNote.UnverifiedSigs, extraSigs...),
	}

	// Re-sign (no new signers) — this just serializes with existing sigs.
	result, err := note.Sign(mergedNote)
	if err != nil {
		return nil, fmt.Errorf("serialize merged note: %w", err)
	}
	return result, nil
}

// ExtractCosignatures extracts signature lines from a cosigned note (excluding the operator).
func ExtractCosignatures(cosigned string, operatorName string) []string {
	parts := strings.SplitN(cosigned, "\n\n", 2)
	if len(parts) < 2 {
		return nil
	}
	var out []string
	for _, line := range strings.Split(parts[1], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "— ") {
			continue
		}
		// Skip the operator's own signature line.
		if strings.HasPrefix(line[2:], operatorName+" ") {
			continue
		}
		out = append(out, line)
	}
	return out
}
