package test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
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

func sampleReceipt() *vur.Receipt {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	return &vur.Receipt{
		ID:             "bench-receipt-001",
		Version:        "1.0.0",
		Tenant:         "bench-tenant",
		Provider:       "openai",
		Model:          "gpt-4o",
		InputTokens:    1000,
		OutputTokens:   500,
		Timestamp:      time.Now().UTC(),
		SignerIdentity: fmt.Sprintf("%x", pub),
	}
}

func BenchmarkCanonicalBytes(b *testing.B) {
	r := sampleReceipt()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.CanonicalBytes()
	}
}

func BenchmarkEd25519Sign(b *testing.B) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	r := sampleReceipt()
	cb, _ := r.CanonicalBytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ed25519.Sign(priv, cb)
	}
}

func BenchmarkEd25519Verify(b *testing.B) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	r := sampleReceipt()
	vur.SignReceipt(r, priv)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vur.VerifyReceipt(r, pub)
	}
}

func BenchmarkMerkleRoot1k(b *testing.B) {
	leaves := makeLeaves(1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = merkle.BuildRoot(leaves)
	}
}

func BenchmarkMerkleRoot10k(b *testing.B) {
	leaves := makeLeaves(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = merkle.BuildRoot(leaves)
	}
}

func BenchmarkMerkleRoot100k(b *testing.B) {
	leaves := makeLeaves(100000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = merkle.BuildRoot(leaves)
	}
}

func BenchmarkSQLiteInsert(b *testing.B) {
	st, err := store.Open(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer st.Close()

	r := *sampleReceipt()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ID = fmt.Sprintf("bench-%d", i)
		_, _ = st.AppendReceipt(r)
	}
}

func BenchmarkFullIngestFlow(b *testing.B) {
	st, err := store.Open(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	defer st.Close()

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	builder := checkpoint.NewBuilder(st, priv, 10000)
	reg := provider.NewRegistry()
	srv := api.New(st, builder, pub, priv, reg, "bench")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	respBody, _ := json.Marshal(map[string]any{
		"id":    "bench-req",
		"usage": map[string]any{"prompt_tokens": 100, "completion_tokens": 50},
	})
	body, _ := json.Marshal(map[string]any{
		"provider":      "openai",
		"tenant":        "bench",
		"model":         "gpt-4o",
		"response_body": json.RawMessage(respBody),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Post(ts.URL+"/v1/receipts",
			"application/json", bytes.NewReader(body))
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
