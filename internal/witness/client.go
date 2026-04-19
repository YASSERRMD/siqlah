package witness

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/sumdb/note"
)

// WitnessClient fetches and submits C2SP checkpoints and cosignatures.
type WitnessClient struct {
	http    *http.Client
	BaseURL string
}

// NewWitnessClient creates a WitnessClient targeting the given siqlah ledger base URL.
func NewWitnessClient(baseURL string) *WitnessClient {
	return &WitnessClient{
		http:    &http.Client{Timeout: 30 * time.Second},
		BaseURL: strings.TrimRight(baseURL, "/"),
	}
}

// FetchCheckpoint retrieves the latest C2SP signed checkpoint from the ledger.
func (c *WitnessClient) FetchCheckpoint() (*C2SPCheckpoint, error) {
	resp, err := c.http.Get(c.BaseURL + "/v1/witness/checkpoint")
	if err != nil {
		return nil, fmt.Errorf("fetch checkpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch checkpoint: HTTP %d: %s", resp.StatusCode, body)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read checkpoint body: %w", err)
	}

	cp, err := ParseCheckpoint(string(raw))
	if err != nil {
		return nil, err
	}
	return cp, nil
}

// VerifyAndCosign verifies the operator's signature on the checkpoint and returns
// a cosigned version signed by witnessKey.
func (c *WitnessClient) VerifyAndCosign(
	raw string,
	operatorVerifier note.Verifier,
	witnessKey note.Signer,
) (string, error) {
	// Verify the operator's signature.
	n, err := note.Open([]byte(raw), note.VerifierList(operatorVerifier))
	if err != nil {
		return "", fmt.Errorf("operator signature invalid: %w", err)
	}

	// Re-sign with the witness key appended.
	cosigned, err := note.Sign(n, witnessKey)
	if err != nil {
		return "", fmt.Errorf("cosign: %w", err)
	}
	return string(cosigned), nil
}

// SubmitCosignature sends a cosigned checkpoint to the ledger.
func (c *WitnessClient) SubmitCosignature(cosignedCheckpoint string) error {
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v1/witness/cosign",
		bytes.NewBufferString(cosignedCheckpoint))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("submit cosignature: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("submit cosignature: HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}

// FetchCosignedCheckpoint retrieves the latest checkpoint with all cosignatures.
func (c *WitnessClient) FetchCosignedCheckpoint() (string, error) {
	resp, err := c.http.Get(c.BaseURL + "/v1/witness/cosigned-checkpoint")
	if err != nil {
		return "", fmt.Errorf("fetch cosigned checkpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("fetch cosigned checkpoint: HTTP %d: %s", resp.StatusCode, body)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return string(raw), nil
}
