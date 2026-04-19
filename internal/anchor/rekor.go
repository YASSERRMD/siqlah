// Package anchor provides Rekor v2 public anchoring for siqlah checkpoints.
package anchor

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultRekorURL = "https://rekor.sigstore.dev"
	rekorTimeout    = 30 * time.Second
)

// AnchorResult holds the result of anchoring a checkpoint to Rekor.
type AnchorResult struct {
	LogIndex  int64
	EntryUUID string
	EntryURL  string
}

// RekorAnchor submits siqlah checkpoint roots to the Rekor transparency log.
type RekorAnchor struct {
	rekorURL string
	priv     *ecdsa.PrivateKey
	pubPEM   []byte
	client   *http.Client
}

// NewRekorAnchor creates a RekorAnchor with an ephemeral ECDSA P-256 signing key.
// Use WithKey to supply a persistent key instead.
func NewRekorAnchor(rekorURL string) (*RekorAnchor, error) {
	if rekorURL == "" {
		rekorURL = DefaultRekorURL
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate anchor key: %w", err)
	}
	return newAnchor(rekorURL, priv)
}

// WithKey creates a RekorAnchor from an existing ECDSA private key.
func WithKey(rekorURL string, priv *ecdsa.PrivateKey) (*RekorAnchor, error) {
	if rekorURL == "" {
		rekorURL = DefaultRekorURL
	}
	return newAnchor(rekorURL, priv)
}

func newAnchor(rekorURL string, priv *ecdsa.PrivateKey) (*RekorAnchor, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return &RekorAnchor{
		rekorURL: rekorURL,
		priv:     priv,
		pubPEM:   pubPEM,
		client:   &http.Client{Timeout: rekorTimeout},
	}, nil
}

// Anchor signs checkpoint bytes and submits them to Rekor as a hashedrekord entry.
func (a *RekorAnchor) Anchor(checkpoint []byte) (*AnchorResult, error) {
	digest := sha256.Sum256(checkpoint)
	sig, err := ecdsa.SignASN1(rand.Reader, a.priv, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign checkpoint: %w", err)
	}

	entry := map[string]any{
		"kind":       "hashedrekord",
		"apiVersion": "0.0.1",
		"spec": map[string]any{
			"signature": map[string]any{
				"content": base64.StdEncoding.EncodeToString(sig),
				"publicKey": map[string]string{
					"content": base64.StdEncoding.EncodeToString(a.pubPEM),
				},
			},
			"data": map[string]any{
				"hash": map[string]string{
					"algorithm": "sha256",
					"value":     hex.EncodeToString(digest[:]),
				},
			},
		},
	}

	body, _ := json.Marshal(entry)
	resp, err := a.client.Post(a.rekorURL+"/api/v1/log/entries", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("post to rekor: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("rekor returned %d: %s", resp.StatusCode, respBody)
	}

	// Response is a map[uuid]entry; extract first entry.
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(respBody, &entries); err != nil {
		return nil, fmt.Errorf("decode rekor response: %w", err)
	}

	for uuid, raw := range entries {
		var ent struct {
			LogIndex int64 `json:"logIndex"`
		}
		if err := json.Unmarshal(raw, &ent); err != nil {
			return nil, fmt.Errorf("decode entry: %w", err)
		}
		return &AnchorResult{
			LogIndex:  ent.LogIndex,
			EntryUUID: uuid,
			EntryURL:  a.rekorURL + "/api/v1/log/entries/" + uuid,
		}, nil
	}
	return nil, errors.New("rekor returned empty entries map")
}

// VerifyAnchor fetches a Rekor entry and verifies the checkpoint hash matches.
func (a *RekorAnchor) VerifyAnchor(checkpoint []byte, logIndex int64) error {
	url := fmt.Sprintf("%s/api/v1/log/entries?logIndex=%d", a.rekorURL, logIndex)
	resp, err := a.client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch rekor entry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no entry at log index %d", logIndex)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("rekor returned %d: %s", resp.StatusCode, b)
	}

	var entries map[string]struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return fmt.Errorf("decode rekor entries: %w", err)
	}

	digest := sha256.Sum256(checkpoint)
	want := hex.EncodeToString(digest[:])

	for _, ent := range entries {
		bodyBytes, err := base64.StdEncoding.DecodeString(ent.Body)
		if err != nil {
			continue
		}
		var body struct {
			Spec struct {
				Data struct {
					Hash struct {
						Value string `json:"value"`
					} `json:"hash"`
				} `json:"data"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			continue
		}
		if body.Spec.Data.Hash.Value == want {
			return nil
		}
	}
	return fmt.Errorf("checkpoint hash %s not found in entry at log index %d", want, logIndex)
}
