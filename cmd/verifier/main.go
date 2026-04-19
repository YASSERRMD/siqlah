// Command verifier is the siqlah client-side verifier CLI.
package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/yasserrmd/siqlah/internal/merkle"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "verify-receipt":
		err = runVerifyReceipt(os.Args[2:])
	case "verify-tokens":
		err = runVerifyTokens(os.Args[2:])
	case "check-proof":
		err = runCheckProof(os.Args[2:])
	case "reconcile":
		err = runReconcile(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `siqlah verifier - client-side receipt and proof verification

Subcommands:
  verify-receipt --receipt PATH --pub HEX
                                Verify a single receipt's Ed25519 signature
  verify-tokens  --receipt PATH
                                Report provider-claimed vs verified token counts
  check-proof    --receipt PATH --ledger URL [--op-pub HEX]
                                Fetch inclusion proof and verify against checkpoint root
  reconcile      --ledger URL --local-log PATH [--threshold PCT]
                                Compare local usage log against ledger, report discrepancies`)
}

// --- verify-receipt ---

func runVerifyReceipt(args []string) error {
	fs := flag.NewFlagSet("verify-receipt", flag.ContinueOnError)
	receiptPath := fs.String("receipt", "", "path to receipt JSON file (required)")
	pubHex := fs.String("pub", "", "operator public key hex (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *receiptPath == "" || *pubHex == "" {
		fs.Usage()
		return errors.New("--receipt and --pub are required")
	}

	r, err := loadReceipt(*receiptPath)
	if err != nil {
		return err
	}
	pub, err := hexToBytes(*pubHex)
	if err != nil {
		return fmt.Errorf("parse --pub: %w", err)
	}
	if len(pub) != 32 {
		return fmt.Errorf("expected 32-byte ed25519 public key, got %d bytes", len(pub))
	}

	if err := vur.VerifyReceipt(r, pub); err != nil {
		fmt.Printf("receipt %s: INVALID (%v)\n", r.ID, err)
		return err
	}
	fmt.Printf("receipt %s: VALID\n", r.ID)
	fmt.Printf("  provider:      %s\n", r.Provider)
	fmt.Printf("  model:         %s\n", r.Model)
	fmt.Printf("  input_tokens:  %d\n", r.InputTokens)
	fmt.Printf("  output_tokens: %d\n", r.OutputTokens)
	fmt.Printf("  timestamp:     %s\n", r.Timestamp.String())
	return nil
}

// --- verify-tokens ---

func runVerifyTokens(args []string) error {
	fs := flag.NewFlagSet("verify-tokens", flag.ContinueOnError)
	receiptPath := fs.String("receipt", "", "path to receipt JSON file (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *receiptPath == "" {
		fs.Usage()
		return errors.New("--receipt is required")
	}

	r, err := loadReceipt(*receiptPath)
	if err != nil {
		return err
	}

	fmt.Printf("receipt %s\n", r.ID)
	fmt.Printf("  provider:              %s\n", r.Provider)
	fmt.Printf("  model:                 %s\n", r.Model)
	fmt.Printf("  provider_input_tokens: %d\n", r.InputTokens)
	fmt.Printf("  provider_output_tokens:%d\n", r.OutputTokens)

	if r.VerifiedInputTokens > 0 || r.VerifiedOutputTokens > 0 {
		fmt.Printf("  verified_input_tokens: %d\n", r.VerifiedInputTokens)
		fmt.Printf("  verified_output_tokens:%d\n", r.VerifiedOutputTokens)
		discIn := discrepancy(r.InputTokens, r.VerifiedInputTokens)
		discOut := discrepancy(r.OutputTokens, r.VerifiedOutputTokens)
		fmt.Printf("  input_discrepancy_pct: %.2f%%\n", discIn)
		fmt.Printf("  output_discrepancy_pct:%.2f%%\n", discOut)
		if discIn > 5 || discOut > 5 {
			fmt.Println("  STATUS: DISCREPANCY DETECTED")
		} else {
			fmt.Println("  STATUS: OK")
		}
	} else {
		fmt.Println("  verified_tokens: not available (tokenizer verification not run)")
	}
	return nil
}

// --- check-proof ---

func runCheckProof(args []string) error {
	fs := flag.NewFlagSet("check-proof", flag.ContinueOnError)
	receiptPath := fs.String("receipt", "", "path to receipt JSON file (required)")
	ledger := fs.String("ledger", "", "siqlah server URL (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *receiptPath == "" || *ledger == "" {
		fs.Usage()
		return errors.New("--receipt and --ledger are required")
	}

	r, err := loadReceipt(*receiptPath)
	if err != nil {
		return err
	}

	proofResp, err := fetchInclusionProof(*ledger, r.ID)
	if err != nil {
		return fmt.Errorf("fetch proof: %w", err)
	}

	// Rebuild the leaf hash from canonical receipt bytes.
	cb, err := r.CanonicalBytes()
	if err != nil {
		return fmt.Errorf("canonical bytes: %w", err)
	}
	leafHash := merkle.HashLeaf(cb)

	// Parse proof hashes.
	root, err := merkle.ParseRoot(proofResp.RootHex)
	if err != nil {
		return fmt.Errorf("parse root: %w", err)
	}
	path, err := parseHexHashes(proofResp.Proof)
	if err != nil {
		return fmt.Errorf("parse proof path: %w", err)
	}

	valid := merkle.VerifyInclusion(leafHash, root, proofResp.LeafIndex, proofResp.TreeSize, path)
	if !valid {
		fmt.Printf("receipt %s: PROOF INVALID\n", r.ID)
		return errors.New("inclusion proof verification failed")
	}
	fmt.Printf("receipt %s: PROOF VALID\n", r.ID)
	fmt.Printf("  checkpoint_id: %d\n", proofResp.CheckpointID)
	fmt.Printf("  leaf_index:    %d / %d\n", proofResp.LeafIndex, proofResp.TreeSize)
	fmt.Printf("  root_hex:      %s\n", proofResp.RootHex[:16]+"...")
	return nil
}

// --- reconcile ---

func runReconcile(args []string) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	ledger := fs.String("ledger", "", "siqlah server URL (required)")
	localLog := fs.String("local-log", "", "path to local usage log JSON file (required)")
	threshold := fs.Float64("threshold", 1.0, "discrepancy threshold percent")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ledger == "" || *localLog == "" {
		fs.Usage()
		return errors.New("--ledger and --local-log are required")
	}

	data, err := os.ReadFile(*localLog)
	if err != nil {
		return fmt.Errorf("read local log: %w", err)
	}

	var localReceipts []vur.Receipt
	if err := json.Unmarshal(data, &localReceipts); err != nil {
		return fmt.Errorf("parse local log: %w", err)
	}

	fmt.Printf("reconciling %d local receipts against %s\n", len(localReceipts), *ledger)

	discrepancies := 0
	missing := 0
	for _, local := range localReceipts {
		remote, err := fetchReceiptFromLedger(*ledger, local.ID)
		if err != nil {
			fmt.Printf("  FETCH_ERROR %s: %v\n", local.ID, err)
			missing++
			continue
		}
		if remote == nil {
			fmt.Printf("  MISSING %s (not found on ledger)\n", local.ID)
			missing++
			continue
		}
		inDisc := discrepancy(local.InputTokens, remote.InputTokens)
		outDisc := discrepancy(local.OutputTokens, remote.OutputTokens)
		if inDisc > *threshold || outDisc > *threshold {
			fmt.Printf("  DISCREPANCY %s: local_in=%d remote_in=%d (%.2f%%) local_out=%d remote_out=%d (%.2f%%)\n",
				local.ID, local.InputTokens, remote.InputTokens, inDisc,
				local.OutputTokens, remote.OutputTokens, outDisc)
			discrepancies++
		}
	}

	fmt.Printf("\nsummary: %d receipts checked, %d missing, %d discrepancies (threshold %.1f%%)\n",
		len(localReceipts), missing, discrepancies, *threshold)
	if discrepancies > 0 || missing > 0 {
		return fmt.Errorf("%d discrepancies, %d missing", discrepancies, missing)
	}
	return nil
}

// --- HTTP helpers ---

type inclusionProofResp struct {
	ReceiptID    string   `json:"receipt_id"`
	CheckpointID int64    `json:"checkpoint_id"`
	LeafIndex    int      `json:"leaf_index"`
	TreeSize     int      `json:"tree_size"`
	RootHex      string   `json:"root_hex"`
	Proof        []string `json:"proof"`
}

func fetchInclusionProof(ledger, receiptID string) (*inclusionProofResp, error) {
	resp, err := http.Get(ledger + "/v1/receipts/" + receiptID + "/proof")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var p inclusionProofResp
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func fetchReceiptFromLedger(ledger, receiptID string) (*vur.Receipt, error) {
	resp, err := http.Get(ledger + "/v1/receipts/" + receiptID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var r vur.Receipt
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// --- helpers ---

func loadReceipt(path string) (*vur.Receipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read receipt file: %w", err)
	}
	var r vur.Receipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse receipt: %w", err)
	}
	return &r, nil
}

func hexToBytes(h string) ([]byte, error) {
	return hex.DecodeString(h)
}

func parseHexHashes(hexes []string) ([][32]byte, error) {
	out := make([][32]byte, len(hexes))
	for i, h := range hexes {
		parsed, err := merkle.ParseRoot(h)
		if err != nil {
			return nil, fmt.Errorf("parse hash[%d] %q: %w", i, h, err)
		}
		out[i] = parsed
	}
	return out, nil
}

func discrepancy(a, b int64) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	if a == 0 {
		return 100
	}
	d := float64(a-b)
	if d < 0 {
		d = -d
	}
	return d / float64(a) * 100
}

func strconv64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

var _ = strconv64 // suppress unused warning
