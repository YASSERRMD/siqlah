package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/yasserrmd/siqlah/internal/merkle"
	"github.com/yasserrmd/siqlah/internal/store"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

// InclusionProofResponse is returned by GET /v1/receipts/{id}/proof.
type InclusionProofResponse struct {
	ReceiptID    string   `json:"receipt_id"`
	CheckpointID int64    `json:"checkpoint_id"`
	LeafIndex    int      `json:"leaf_index"`
	TreeSize     int      `json:"tree_size"`
	RootHex      string   `json:"root_hex"`
	Proof        []string `json:"proof"`
}

// ConsistencyProofResponse is returned by GET /v1/checkpoints/{id}/consistency/{old_id}.
type ConsistencyProofResponse struct {
	OldCheckpointID int64    `json:"old_checkpoint_id"`
	NewCheckpointID int64    `json:"new_checkpoint_id"`
	OldSize         int      `json:"old_size"`
	NewSize         int      `json:"new_size"`
	OldRootHex      string   `json:"old_root_hex"`
	NewRootHex      string   `json:"new_root_hex"`
	Proof           []string `json:"proof"`
}

func (s *Server) handleInclusionProof(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sr, err := s.store.GetReceiptByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch receipt: "+err.Error())
		return
	}
	if sr == nil {
		writeError(w, http.StatusNotFound, "receipt not found")
		return
	}

	cp, err := findCheckpointForRow(s.store, sr.RowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "find checkpoint: "+err.Error())
		return
	}
	if cp == nil {
		writeError(w, http.StatusNotFound, "receipt has not been batched into a checkpoint yet")
		return
	}

	receipts, err := s.store.GetReceiptsByRange(cp.BatchStart, cp.BatchEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch receipts: "+err.Error())
		return
	}

	leaves, index, err := buildLeavesAndIndex(receipts, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build leaves: "+err.Error())
		return
	}

	path, err := merkle.InclusionProof(leaves, index)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "inclusion proof: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, InclusionProofResponse{
		ReceiptID:    id,
		CheckpointID: cp.ID,
		LeafIndex:    index,
		TreeSize:     cp.TreeSize,
		RootHex:      cp.RootHex,
		Proof:        hexSlice(path),
	})
}

func (s *Server) handleConsistencyProof(w http.ResponseWriter, r *http.Request) {
	newID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	oldIDStr := r.PathValue("old_id")
	oldID, err := strconv.ParseInt(oldIDStr, 10, 64)
	if err != nil || oldID <= 0 {
		writeError(w, http.StatusBadRequest, "old_id must be a positive integer")
		return
	}

	newCP, err := s.store.GetCheckpoint(newID)
	if err != nil || newCP == nil {
		writeError(w, http.StatusNotFound, "new checkpoint not found")
		return
	}
	oldCP, err := s.store.GetCheckpoint(oldID)
	if err != nil || oldCP == nil {
		writeError(w, http.StatusNotFound, "old checkpoint not found")
		return
	}

	if oldCP.TreeSize >= newCP.TreeSize {
		writeError(w, http.StatusBadRequest, "old_id must refer to a smaller tree than id")
		return
	}

	receipts, err := s.store.GetReceiptsByRange(oldCP.BatchStart, newCP.BatchEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch receipts: "+err.Error())
		return
	}

	leaves, err := leavesFromReceipts(receipts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build leaves: "+err.Error())
		return
	}

	if len(leaves) < newCP.TreeSize {
		writeError(w, http.StatusInternalServerError,
			fmt.Sprintf("expected %d leaves, got %d", newCP.TreeSize, len(leaves)))
		return
	}

	proof, err := merkle.ConsistencyProof(oldCP.TreeSize, newCP.TreeSize, leaves[:newCP.TreeSize])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "consistency proof: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, ConsistencyProofResponse{
		OldCheckpointID: oldID,
		NewCheckpointID: newID,
		OldSize:         oldCP.TreeSize,
		NewSize:         newCP.TreeSize,
		OldRootHex:      oldCP.RootHex,
		NewRootHex:      newCP.RootHex,
		Proof:           hexSlice(proof),
	})
}

// findCheckpointForRow pages through checkpoints to find the one covering rowID.
func findCheckpointForRow(st store.Store, rowID int64) (*store.Checkpoint, error) {
	offset := 0
	for {
		batch, err := st.ListCheckpoints(offset, 50)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return nil, nil
		}
		for i := range batch {
			cp := &batch[i]
			if cp.BatchStart <= rowID && rowID <= cp.BatchEnd {
				return cp, nil
			}
		}
		offset += len(batch)
	}
}

// buildLeavesAndIndex builds leaf hashes from receipts and returns the index
// of the receipt with the given UUID.
func buildLeavesAndIndex(receipts []vur.Receipt, targetID string) ([][32]byte, int, error) {
	leaves := make([][32]byte, len(receipts))
	index := -1
	for i, r := range receipts {
		cb, err := r.CanonicalBytes()
		if err != nil {
			return nil, 0, fmt.Errorf("canonical bytes for receipt %s: %w", r.ID, err)
		}
		leaves[i] = merkle.HashLeaf(cb)
		if r.ID == targetID {
			index = i
		}
	}
	if index < 0 {
		return nil, 0, fmt.Errorf("receipt %s not found in checkpoint range", targetID)
	}
	return leaves, index, nil
}

// leavesFromReceipts builds Merkle leaf hashes from a slice of receipts.
func leavesFromReceipts(receipts []vur.Receipt) ([][32]byte, error) {
	leaves := make([][32]byte, len(receipts))
	for i, r := range receipts {
		cb, err := r.CanonicalBytes()
		if err != nil {
			return nil, fmt.Errorf("canonical bytes for receipt %s: %w", r.ID, err)
		}
		leaves[i] = merkle.HashLeaf(cb)
	}
	return leaves, nil
}

// hexSlice converts a slice of [32]byte to hex strings.
func hexSlice(hashes [][32]byte) []string {
	out := make([]string, len(hashes))
	for i, h := range hashes {
		out[i] = merkle.FormatRoot(h)
	}
	return out
}
