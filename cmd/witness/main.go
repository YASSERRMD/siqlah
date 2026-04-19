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
	"github.com/yasserrmd/siqlah/internal/witness"
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
  cosign --ledger URL --key PATH --op-pub HEX [--origin STR] [--witness-name NAME] [--legacy --cp ID]
                                          Fetch latest C2SP checkpoint, verify operator sig, and cosign
  verify --ledger URL --op-pub HEX [--origin STR] [--threshold N] [--legacy --cp ID]
                                          Verify a checkpoint's operator and witness signatures
  watch  --ledger URL --op-pub HEX --key PATH [--origin STR] [--interval DURATION]
                                          Continuously watch for new checkpoints and auto-cosign (C2SP)`)
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
	keyPath := fs.String("key", "", "path to private key hex file (required)")
	opPubHex := fs.String("op-pub", "", "operator public key hex (required)")
	origin := fs.String("origin", witness.DefaultOrigin, "C2SP log origin string")
	witnessName := fs.String("witness-name", "", "witness name for note signing (defaults to public key hex)")
	// Legacy mode flags
	legacy := fs.Bool("legacy", false, "use legacy checkpoint cosign instead of C2SP")
	cpID := fs.Int64("cp", 0, "checkpoint ID (legacy mode only)")
	witnessID := fs.String("witness-id", "", "witness identifier (legacy mode only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ledger == "" || *keyPath == "" || *opPubHex == "" {
		fs.Usage()
		return errors.New("--ledger, --key, --op-pub are required")
	}

	priv, pub, err := loadPrivKey(*keyPath)
	if err != nil {
		return err
	}
	opPub, err := hexToEd25519Pub(*opPubHex)
	if err != nil {
		return fmt.Errorf("parse op-pub: %w", err)
	}

	if *legacy {
		// Legacy mode: cosign a specific checkpoint by ID.
		if *cpID == 0 {
			return errors.New("--cp is required in --legacy mode")
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
		return nil
	}

	// C2SP mode.
	name := *witnessName
	if name == "" {
		name = hex.EncodeToString(pub)
	}

	client := witness.NewWitnessClient(*ledger)
	cp, err := client.FetchCheckpoint()
	if err != nil {
		return fmt.Errorf("fetch C2SP checkpoint: %w", err)
	}
	fmt.Printf("fetched checkpoint: origin=%s tree_size=%d\n", cp.Origin, cp.TreeSize)

	operatorVerifier, err := witness.NewNoteVerifier(*origin, opPub)
	if err != nil {
		return fmt.Errorf("create operator verifier: %w", err)
	}
	witnessKey, err := witness.NewNoteSigner(name, priv)
	if err != nil {
		return fmt.Errorf("create witness signer: %w", err)
	}

	cosigned, err := client.VerifyAndCosign(cp.Raw, operatorVerifier, witnessKey)
	if err != nil {
		return fmt.Errorf("verify and cosign: %w", err)
	}

	if err := client.SubmitCosignature(cosigned); err != nil {
		return fmt.Errorf("submit cosignature: %w", err)
	}

	fmt.Printf("cosigned and submitted C2SP checkpoint as witness %s\n", name)
	return nil
}

// --- verify ---

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	ledger := fs.String("ledger", "", "siqlah server URL (required)")
	opPubHex := fs.String("op-pub", "", "operator public key hex (required)")
	origin := fs.String("origin", witness.DefaultOrigin, "C2SP log origin string")
	threshold := fs.Int("threshold", 0, "minimum required witness cosignatures for C2SP verify")
	checkRekor := fs.Bool("check-rekor", false, "also verify Rekor public anchoring status")
	// Legacy mode flags
	legacy := fs.Bool("legacy", false, "use legacy checkpoint verify instead of C2SP")
	cpID := fs.Int64("cp", 0, "checkpoint ID (legacy mode only)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ledger == "" || *opPubHex == "" {
		fs.Usage()
		return errors.New("--ledger, --op-pub are required")
	}

	opPub, err := hexToEd25519Pub(*opPubHex)
	if err != nil {
		return fmt.Errorf("parse op-pub: %w", err)
	}

	if *legacy {
		if *cpID == 0 {
			return errors.New("--cp is required in --legacy mode")
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
		fmt.Printf("witnesses: %d\n", len(witnesses))
		if *checkRekor && *cpID != 0 {
			vr, err := fetchVerifyResponse(*ledger, *cpID)
			if err != nil {
				fmt.Printf("rekor: error fetching (%v)\n", err)
			} else if vr.RekorAnchored {
				fmt.Printf("rekor: ANCHORED (log_index=%d url=%s)\n", vr.RekorLogIndex, vr.RekorEntryURL)
			} else {
				fmt.Println("rekor: NOT ANCHORED")
			}
		}
		return nil
	}

	// C2SP mode.
	operatorVerifier, err := witness.NewNoteVerifier(*origin, opPub)
	if err != nil {
		return fmt.Errorf("create operator verifier: %w", err)
	}

	client := witness.NewWitnessClient(*ledger)
	rawCosigned, err := client.FetchCosignedCheckpoint()
	if err != nil {
		return fmt.Errorf("fetch cosigned checkpoint: %w", err)
	}

	if err := witness.VerifyCosignedCheckpoint(rawCosigned, operatorVerifier, nil, *threshold); err != nil {
		fmt.Printf("verification FAILED: %v\n", err)
		return err
	}

	cp, err := witness.ParseCheckpoint(rawCosigned)
	if err != nil {
		return err
	}
	fmt.Printf("C2SP checkpoint VALID: origin=%s tree_size=%d threshold=%d\n",
		cp.Origin, cp.TreeSize, *threshold)
	return nil
}

// --- watch ---

func runWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	ledger := fs.String("ledger", "", "siqlah server URL (required)")
	opPubHex := fs.String("op-pub", "", "operator public key hex (required)")
	keyPath := fs.String("key", "", "path to private key hex file (required)")
	interval := fs.Duration("interval", 30*time.Second, "polling interval")
	origin := fs.String("origin", witness.DefaultOrigin, "C2SP log origin string")
	witnessName := fs.String("witness-name", "", "witness name for note signing (defaults to public key hex)")
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

	name := *witnessName
	if name == "" {
		name = hex.EncodeToString(pub)
	}

	operatorVerifier, err := witness.NewNoteVerifier(*origin, opPub)
	if err != nil {
		return fmt.Errorf("create operator verifier: %w", err)
	}
	witnessKey, err := witness.NewNoteSigner(name, priv)
	if err != nil {
		return fmt.Errorf("create witness signer: %w", err)
	}

	client := witness.NewWitnessClient(*ledger)
	fmt.Printf("watching %s every %s as witness %s (C2SP)\n", *ledger, *interval, name)

	var lastTreeSize uint64
	for {
		cp, err := client.FetchCheckpoint()
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch checkpoint: %v\n", err)
		} else if cp.TreeSize > lastTreeSize {
			cosigned, err := client.VerifyAndCosign(cp.Raw, operatorVerifier, witnessKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "verify and cosign: %v\n", err)
			} else if err := client.SubmitCosignature(cosigned); err != nil {
				fmt.Fprintf(os.Stderr, "submit cosignature: %v\n", err)
			} else {
				fmt.Printf("cosigned checkpoint tree_size=%d\n", cp.TreeSize)
				lastTreeSize = cp.TreeSize
			}
		}
		time.Sleep(*interval)
	}
}

// --- HTTP helpers (legacy) ---

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

type verifyResponse struct {
	OperatorValid bool              `json:"operator_valid"`
	Witnesses     map[string]string `json:"witnesses"`
	RekorAnchored bool              `json:"rekor_anchored"`
	RekorLogIndex int64             `json:"rekor_log_index"`
	RekorEntryURL string            `json:"rekor_entry_url"`
}

func fetchVerifyResponse(ledger string, cpID int64) (*verifyResponse, error) {
	resp, err := http.Get(ledger + "/v1/checkpoints/" + strconv.FormatInt(cpID, 10) + "/verify")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var vr verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return nil, err
	}
	return &vr, nil
}

func fetchWitnesses(ledger string, cpID int64) (map[string]string, error) {
	vr, err := fetchVerifyResponse(ledger, cpID)
	if err != nil {
		return nil, err
	}
	return vr.Witnesses, nil
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
