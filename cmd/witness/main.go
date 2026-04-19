// Command witness is the siqlah witness CLI for cosigning checkpoints.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "cosign":
		err = runCosign(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	case "watch":
		err = runWatch(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, `siqlah witness - cosign and verify siqlah checkpoints

Subcommands:
  keygen                                  Generate an Ed25519 keypair
  cosign --ledger URL --cp ID --key PATH --op-pub HEX
                                          Fetch a checkpoint, verify operator sig, and cosign
  verify --ledger URL --cp ID --op-pub HEX
                                          Verify a checkpoint's operator and witness signatures
  watch  --ledger URL --op-pub HEX --key PATH --interval DURATION
                                          Continuously watch for new checkpoints and auto-cosign`)
}

// --- keygen ---

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	out := fs.String("out", "", "write private key hex to file (stdout if empty)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	pubHex := hex.EncodeToString(pub)
	privHex := hex.EncodeToString(priv)

	if *out != "" {
		if err := os.WriteFile(*out, []byte(privHex), 0600); err != nil {
			return fmt.Errorf("write key file: %w", err)
		}
		fmt.Printf("private_key_file: %s\n", *out)
	} else {
		fmt.Printf("private_key_hex: %s\n", privHex)
	}
	fmt.Printf("public_key_hex:  %s\n", pubHex)
	return nil
}

// --- cosign ---

func runCosign(args []string) error {
	fs := flag.NewFlagSet("cosign", flag.ContinueOnError)
	ledger := fs.String("ledger", "", "siqlah server URL (required)")
	cpID := fs.Int64("cp", 0, "checkpoint ID (required)")
	keyPath := fs.String("key", "", "path to private key hex file (required)")
	opPubHex := fs.String("op-pub", "", "operator public key hex (required)")
	witnessID := fs.String("witness-id", "", "witness identifier (defaults to public key hex)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ledger == "" || *cpID == 0 || *keyPath == "" || *opPubHex == "" {
		fs.Usage()
		return errors.New("--ledger, --cp, --key, --op-pub are required")
	}

	priv, pub, err := loadPrivKey(*keyPath)
	if err != nil {
		return err
	}
	opPub, err := hexToEd25519Pub(*opPubHex)
	if err != nil {
		return fmt.Errorf("parse op-pub: %w", err)
	}

	wid := *witnessID
	if wid == "" {
		wid = hex.EncodeToString(pub)
	}

	cp, err := fetchCheckpoint(*ledger, *cpID)
	if err != nil {
		return fmt.Errorf("fetch checkpoint: %w", err)
	}

	sigHex, err := checkpoint.CoSign(*cp, opPub, priv)
	if err != nil {
		return fmt.Errorf("cosign: %w", err)
	}

	if err := submitWitness(*ledger, *cpID, wid, sigHex); err != nil {
		return fmt.Errorf("submit witness: %w", err)
	}

	fmt.Printf("cosigned checkpoint %d as witness %s\n", *cpID, wid)
	fmt.Printf("sig_hex: %s\n", sigHex)
	return nil
}

// --- verify ---

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	ledger := fs.String("ledger", "", "siqlah server URL (required)")
	cpID := fs.Int64("cp", 0, "checkpoint ID (required)")
	opPubHex := fs.String("op-pub", "", "operator public key hex (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ledger == "" || *cpID == 0 || *opPubHex == "" {
		fs.Usage()
		return errors.New("--ledger, --cp, --op-pub are required")
	}

	opPub, err := hexToEd25519Pub(*opPubHex)
	if err != nil {
		return fmt.Errorf("parse op-pub: %w", err)
	}

	cp, err := fetchCheckpoint(*ledger, *cpID)
	if err != nil {
		return fmt.Errorf("fetch checkpoint: %w", err)
	}

	fmt.Printf("checkpoint %d: tree_size=%d root=%s\n", cp.ID, cp.TreeSize, cp.RootHex)

	if err := checkpoint.VerifyOperatorSignature(*cp, opPub); err != nil {
		fmt.Printf("operator_sig: INVALID (%v)\n", err)
	} else {
		fmt.Printf("operator_sig: VALID\n")
	}

	witnesses, err := fetchWitnesses(*ledger, *cpID)
	if err != nil {
		fmt.Printf("witnesses: error fetching (%v)\n", err)
		return nil
	}
	if len(witnesses) == 0 {
		fmt.Println("witnesses: none")
		return nil
	}
	fmt.Printf("witnesses: %d\n", len(witnesses))
	for wid, sig := range witnesses {
		fmt.Printf("  %s: %s\n", wid, sig[:min(16, len(sig))]+"...")
	}
	return nil
}

// --- watch ---

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	ledger := fs.String("ledger", "", "siqlah server URL (required)")
	opPubHex := fs.String("op-pub", "", "operator public key hex (required)")
	keyPath := fs.String("key", "", "path to private key hex file (required)")
	interval := fs.Duration("interval", 30*time.Second, "polling interval")
	witnessID := fs.String("witness-id", "", "witness identifier (defaults to public key hex)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ledger == "" || *opPubHex == "" || *keyPath == "" {
		fs.Usage()
		return errors.New("--ledger, --op-pub, --key are required")
	}

	priv, pub, err := loadPrivKey(*keyPath)
	if err != nil {
		return err
	}
	opPub, err := hexToEd25519Pub(*opPubHex)
	if err != nil {
		return fmt.Errorf("parse op-pub: %w", err)
	}

	wid := *witnessID
	if wid == "" {
		wid = hex.EncodeToString(pub)
	}

	fmt.Printf("watching %s every %s as witness %s\n", *ledger, *interval, wid)

	lastSigned := int64(0)
	for {
		cps, err := listCheckpoints(*ledger, 0, 50)
		if err != nil {
			fmt.Fprintf(os.Stderr, "list checkpoints: %v\n", err)
		} else {
			for i := len(cps) - 1; i >= 0; i-- {
				cp := &cps[i]
				if cp.ID <= lastSigned {
					continue
				}
				sigHex, err := checkpoint.CoSign(*cp, opPub, priv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "cosign cp %d: %v\n", cp.ID, err)
					continue
				}
				if err := submitWitness(*ledger, cp.ID, wid, sigHex); err != nil {
					fmt.Fprintf(os.Stderr, "submit cp %d: %v\n", cp.ID, err)
					continue
				}
				fmt.Printf("cosigned checkpoint %d\n", cp.ID)
				lastSigned = cp.ID
			}
		}
		time.Sleep(*interval)
	}
}

// --- HTTP helpers ---

func fetchCheckpoint(ledger string, id int64) (*store.Checkpoint, error) {
	resp, err := http.Get(ledger + "/v1/checkpoints/" + strconv.FormatInt(id, 10))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var cp store.Checkpoint
	if err := json.NewDecoder(resp.Body).Decode(&cp); err != nil {
		return nil, fmt.Errorf("decode checkpoint: %w", err)
	}
	return &cp, nil
}

func fetchWitnesses(ledger string, cpID int64) (map[string]string, error) {
	resp, err := http.Get(ledger + "/v1/checkpoints/" + strconv.FormatInt(cpID, 10) + "/verify")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var vresp struct {
		Witnesses map[string]string `json:"witnesses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vresp); err != nil {
		return nil, err
	}
	return vresp.Witnesses, nil
}

func submitWitness(ledger string, cpID int64, witnessID, sigHex string) error {
	body, _ := json.Marshal(map[string]string{
		"witness_id": witnessID,
		"sig_hex":    sigHex,
	})
	resp, err := http.Post(
		ledger+"/v1/checkpoints/"+strconv.FormatInt(cpID, 10)+"/witness",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, b)
	}
	return nil
}

type checkpointList struct {
	Checkpoints []store.Checkpoint `json:"checkpoints"`
}

func listCheckpoints(ledger string, offset, limit int) ([]store.Checkpoint, error) {
	url := fmt.Sprintf("%s/v1/checkpoints?offset=%d&limit=%d", ledger, offset, limit)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var cl checkpointList
	if err := json.NewDecoder(resp.Body).Decode(&cl); err != nil {
		return nil, err
	}
	return cl.Checkpoints, nil
}

// --- key helpers ---

func loadPrivKey(path string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read key file: %w", err)
	}
	privHex := string(bytes.TrimSpace(data))
	privBytes, err := hex.DecodeString(privHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		return nil, nil, fmt.Errorf("expected %d-byte private key, got %d", ed25519.PrivateKeySize, len(privBytes))
	}
	priv := ed25519.PrivateKey(privBytes)
	pub := priv.Public().(ed25519.PublicKey)
	return priv, pub, nil
}

func hexToEd25519Pub(h string) (ed25519.PublicKey, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d-byte public key, got %d", ed25519.PublicKeySize, len(b))
	}
	return ed25519.PublicKey(b), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
