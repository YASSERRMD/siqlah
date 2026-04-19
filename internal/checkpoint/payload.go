package checkpoint

import (
	"encoding/json"
	"fmt"
)

// SignedPayload is the canonical representation that gets signed by the operator.
type SignedPayload struct {
	BatchStart      int64  `json:"batch_start"`
	BatchEnd        int64  `json:"batch_end"`
	TreeSize        int    `json:"tree_size"`
	RootHex         string `json:"root_hex"`
	PreviousRootHex string `json:"previous_root_hex"`
	IssuedAt        string `json:"issued_at"` // RFC3339Nano UTC
}

// Bytes returns deterministic JSON bytes of the payload suitable for signing.
func (p *SignedPayload) Bytes() ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("payload bytes: %w", err)
	}
	return b, nil
}
