package test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	mathrand "math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yasserrmd/siqlah/internal/api"
	"github.com/yasserrmd/siqlah/internal/checkpoint"
	"github.com/yasserrmd/siqlah/internal/merkle"
	"github.com/yasserrmd/siqlah/internal/provider"
	"github.com/yasserrmd/siqlah/internal/store"
	"github.com/yasserrmd/siqlah/pkg/vur"
)

func newE2EServer(t *testing.T) (*httptest.Server, *store.SQLiteStore, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	builder := checkpoint.NewBuilder(st, priv, 1000)
	reg := provider.NewRegistry()
	srv := api.New(st, builder, pub, priv, reg, "test")
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, st, pub, priv
}

func ingestReceipt(t *testing.T, baseURL, provider, tenant, model string, inputTokens, outputTokens int) string {
	t.Helper()
	respBody, _ := json.Marshal(map[string]any{
		"id": fmt.Sprintf("req-%d", time.Now().UnixNano()),
		"usage": map[string]any{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
		},
	})
	body, _ := json.Marshal(map[string]any{
		"provider":      provider,
		"tenant":        tenant,
		"model":         model,
		"response_body": json.RawMessage(respBody),
	})
	resp, err := http.Post(baseURL+"/v1/receipts", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("ingest: expected 201, got %d", resp.StatusCode)
	}
	var r map[string]any
	json.NewDecoder(resp.Body).Decode(&r)
	return r["id"].(string)
}

func TestE2EFullFlow(t *testing.T) {
	ts, _, opPub, _ := newE2EServer(t)

	// 1. Ingest 100 receipts.
	const numReceipts = 100
	receiptIDs := make([]string, numReceipts)
	for i := 0; i < numReceipts; i++ {
		receiptIDs[i] = ingestReceipt(t, ts.URL, "openai", "e2e-tenant", "gpt-4o",
			100+i, 50+i)
	}
	t.Logf("ingested %d receipts", numReceipts)

	// 2. Build checkpoint.
	resp, err := http.Post(ts.URL+"/v1/checkpoints/build", "application/json", nil)
	if err != nil {
		t.Fatalf("build checkpoint: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("build checkpoint: expected 201, got %d", resp.StatusCode)
	}
	var cpResp map[string]any
	json.NewDecoder(resp.Body).Decode(&cpResp)
	resp.Body.Close()

	rootHex, _ := cpResp["RootHex"].(string)
	if rootHex == "" {
		t.Fatal("checkpoint has no RootHex")
	}
	t.Logf("checkpoint built: root=%s...", rootHex[:16])

	// 3. Verify checkpoint operator signature.
	vresp, err := http.Get(ts.URL + "/v1/checkpoints/1/verify")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	var verify map[string]any
	json.NewDecoder(vresp.Body).Decode(&verify)
	vresp.Body.Close()
	if verify["operator_valid"] != true {
		t.Fatalf("operator_valid should be true, got %v", verify["operator_valid"])
	}

	// 4. Simulate witness cosigning.
	witnessPub, witnessPriv, _ := ed25519.GenerateKey(rand.Reader)
	_ = witnessPub

	// Get checkpoint from store for cosigning.
	getResp, _ := http.Get(ts.URL + "/v1/checkpoints/1")
	var cp store.Checkpoint
	json.NewDecoder(getResp.Body).Decode(&cp)
	getResp.Body.Close()

	sigHex, err := checkpoint.CoSign(cp, opPub, witnessPriv)
	if err != nil {
		t.Fatalf("cosign: %v", err)
	}

	// Submit witness signature.
	wbody, _ := json.Marshal(map[string]string{
		"witness_id": "e2e-witness",
		"sig_hex":    sigHex,
	})
	wresp, _ := http.Post(ts.URL+"/v1/checkpoints/1/witness",
		"application/json", bytes.NewReader(wbody))
	if wresp.StatusCode != http.StatusCreated {
		t.Fatalf("submit witness: expected 201, got %d", wresp.StatusCode)
	}
	wresp.Body.Close()

	// 5. Verify witness signature appears.
	vresp2, _ := http.Get(ts.URL + "/v1/checkpoints/1/verify")
	var verify2 map[string]any
	json.NewDecoder(vresp2.Body).Decode(&verify2)
	vresp2.Body.Close()
	witnesses, _ := verify2["witnesses"].(map[string]any)
	if _, ok := witnesses["e2e-witness"]; !ok {
		t.Error("witness signature not found after submission")
	}

	// 6. Fetch and verify inclusion proof for a random receipt.
	target := receiptIDs[mathrand.Intn(numReceipts)]
	proofResp, err := http.Get(ts.URL + "/v1/receipts/" + target + "/proof")
	if err != nil {
		t.Fatalf("inclusion proof: %v", err)
	}
	var proofData map[string]any
	json.NewDecoder(proofResp.Body).Decode(&proofData)
	proofResp.Body.Close()

	if proofData["root_hex"] == "" {
		t.Error("inclusion proof missing root_hex")
	}
	leafIndex := int(proofData["leaf_index"].(float64))
	treeSize := int(proofData["tree_size"].(float64))
	if treeSize != numReceipts {
		t.Errorf("tree_size should be %d, got %d", numReceipts, treeSize)
	}
	t.Logf("inclusion proof: leaf=%d/%d", leafIndex, treeSize)

	// 7. Verify the inclusion proof locally.
	sr, err := store.Open(":memory:") // we need to get the receipt from the ledger
	_ = sr
	rresp, _ := http.Get(ts.URL + "/v1/receipts/" + target)
	var receipt vur.Receipt
	json.NewDecoder(rresp.Body).Decode(&receipt)
	rresp.Body.Close()

	cb, err := receipt.CanonicalBytes()
	if err != nil {
		t.Fatalf("canonical bytes: %v", err)
	}
	leafHash := merkle.HashLeaf(cb)

	proofHexes, _ := proofData["proof"].([]any)
	path := make([][32]byte, len(proofHexes))
	for i, h := range proofHexes {
		parsed, err := merkle.ParseRoot(h.(string))
		if err != nil {
			t.Fatalf("parse proof[%d]: %v", i, err)
		}
		path[i] = parsed
	}
	root, err := merkle.ParseRoot(proofData["root_hex"].(string))
	if err != nil {
		t.Fatalf("parse root: %v", err)
	}

	if !merkle.VerifyInclusion(leafHash, root, leafIndex, treeSize, path) {
		t.Error("inclusion proof verification failed")
	}
	t.Log("inclusion proof verified successfully")
}

func TestE2EConsistencyProof(t *testing.T) {
	ts, _, _, _ := newE2EServer(t)

	// Ingest 5 receipts and build first checkpoint.
	for i := 0; i < 5; i++ {
		ingestReceipt(t, ts.URL, "openai", "tenant", "gpt-4o", 100, 50)
	}
	resp1, _ := http.Post(ts.URL+"/v1/checkpoints/build", "application/json", nil)
	resp1.Body.Close()

	// Ingest 5 more and build second checkpoint.
	for i := 0; i < 5; i++ {
		ingestReceipt(t, ts.URL, "openai", "tenant", "gpt-4o", 200, 100)
	}
	resp2, _ := http.Post(ts.URL+"/v1/checkpoints/build", "application/json", nil)
	resp2.Body.Close()

	// Fetch consistency proof: checkpoint 2 is consistent with checkpoint 1.
	consResp, err := http.Get(ts.URL + "/v1/checkpoints/2/consistency/1")
	if err != nil {
		t.Fatalf("consistency proof: %v", err)
	}
	if consResp.StatusCode != http.StatusOK {
		t.Fatalf("consistency proof: expected 200, got %d", consResp.StatusCode)
	}
	var consData map[string]any
	json.NewDecoder(consResp.Body).Decode(&consData)
	consResp.Body.Close()

	if consData["old_root_hex"] == "" || consData["new_root_hex"] == "" {
		t.Error("consistency proof missing root hexes")
	}
	t.Logf("consistency proof: old_size=%v new_size=%v proofLen=%v",
		consData["old_size"], consData["new_size"], len(consData["proof"].([]any)))
}

// randInt64 generates a random int64 for use in tests.
func randInt64() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return n.Int64()
}
