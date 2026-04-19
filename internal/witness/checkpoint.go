// Package witness implements the C2SP tlog-witness and tlog-cosignature/v1 protocols.
package witness

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/sumdb/note"
)

// DefaultOrigin is the default log origin string for siqlah.
const DefaultOrigin = "siqlah.dev/log"

// C2SPCheckpoint is a parsed C2SP signed-note checkpoint.
type C2SPCheckpoint struct {
	Origin   string
	TreeSize uint64
	RootHash []byte
	// Raw holds the full signed-note bytes (including signatures).
	Raw string
}

// FormatCheckpoint serializes a checkpoint body (unsigned) in C2SP format:
//
//	<origin>\n<tree_size>\n<root_hash_base64>\n
func FormatCheckpoint(origin string, treeSize uint64, rootHash []byte) string {
	return fmt.Sprintf("%s\n%d\n%s\n",
		origin,
		treeSize,
		base64.StdEncoding.EncodeToString(rootHash),
	)
}

// SignCheckpoint signs the checkpoint body with the provided note.Signer
// and returns the full signed note bytes.
func SignCheckpoint(body string, signer note.Signer) ([]byte, error) {
	n := &note.Note{Text: body}
	return note.Sign(n, signer)
}

// ParseCheckpoint parses a raw signed-note checkpoint string.
// It does not verify signatures; use OpenCheckpoint to verify.
func ParseCheckpoint(raw string) (*C2SPCheckpoint, error) {
	// Split on the double-newline that separates body from signatures.
	parts := strings.SplitN(raw, "\n\n", 2)
	body := parts[0]

	lines := strings.Split(body, "\n")
	if len(lines) < 3 {
		return nil, fmt.Errorf("malformed checkpoint: expected at least 3 body lines, got %d", len(lines))
	}

	origin := lines[0]
	treeSize, err := strconv.ParseUint(lines[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("malformed checkpoint: invalid tree size %q: %w", lines[1], err)
	}
	rootHash, err := base64.StdEncoding.DecodeString(lines[2])
	if err != nil {
		return nil, fmt.Errorf("malformed checkpoint: invalid root hash %q: %w", lines[2], err)
	}

	return &C2SPCheckpoint{
		Origin:   origin,
		TreeSize: treeSize,
		RootHash: rootHash,
		Raw:      raw,
	}, nil
}

// OpenCheckpoint parses and verifies signatures on a signed checkpoint.
// Returns the checkpoint and the verified note (with Sigs populated).
func OpenCheckpoint(raw string, verifiers note.Verifiers) (*C2SPCheckpoint, *note.Note, error) {
	n, err := note.Open([]byte(raw), verifiers)
	if err != nil {
		return nil, nil, fmt.Errorf("open checkpoint note: %w", err)
	}

	cp, err := ParseCheckpoint(n.Text)
	if err != nil {
		return nil, nil, err
	}
	cp.Raw = raw
	return cp, n, nil
}
