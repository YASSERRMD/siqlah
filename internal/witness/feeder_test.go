package witness

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/mod/sumdb/note"
)

// mockLogReader implements LogReader for tests.
type mockLogReader struct {
	rawCP []byte
	proof [][]byte
	err   error
}

func (m *mockLogReader) LatestCheckpoint() ([]byte, error) { return m.rawCP, m.err }
func (m *mockLogReader) ConsistencyProof(_, _ uint64) ([][]byte, error) {
	return m.proof, m.err
}

// mockWitnessServer simulates a minimal tlog-witness /add-checkpoint endpoint.
type mockWitnessServer struct {
	received []byte
	respond  func(w http.ResponseWriter, body []byte)
}

func (m *mockWitnessServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/add-checkpoint") {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	body, _ := io.ReadAll(r.Body)
	m.received = body
	if m.respond != nil {
		m.respond(w, body)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

func makeRawCP(t *testing.T, origin string, size uint64) ([]byte, note.Signer) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewNoteSigner(origin, priv)
	if err != nil {
		t.Fatal(err)
	}
	text := "test-operator/log\n5\nabcdef==\n\n"
	_ = size
	rawCP, err := note.Sign(&note.Note{Text: text}, signer)
	if err != nil {
		t.Fatal(err)
	}
	return rawCP, signer
}

func TestWitnessFeeder_FeedOne_Success(t *testing.T) {
	rawCP, _ := makeRawCP(t, "test-operator", 5)

	mock := &mockWitnessServer{
		respond: func(w http.ResponseWriter, _ []byte) {
			w.WriteHeader(http.StatusOK)
			w.Write(rawCP)
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	wPub, _, _ := ed25519.GenerateKey(rand.Reader)
	wVerifier, err := NewNoteVerifier("mock-witness-1", wPub)
	if err != nil {
		t.Fatal(err)
	}

	lr := &mockLogReader{rawCP: rawCP}
	feeder := NewWitnessFeeder(lr, []ExternalWitness{
		{Name: "mock-witness", URL: srv.URL, Verifier: wVerifier},
	}, 0)

	results := feeder.Feed(context.Background(), rawCP, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}
	if len(results[0].Cosigned) == 0 {
		t.Error("expected non-empty cosigned bytes")
	}
}

func TestWitnessFeeder_FeedOne_HTTPError(t *testing.T) {
	mock := &mockWitnessServer{
		respond: func(w http.ResponseWriter, _ []byte) {
			http.Error(w, "internal error", http.StatusInternalServerError)
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	wPub, _, _ := ed25519.GenerateKey(rand.Reader)
	wVerifier, _ := NewNoteVerifier("mock-witness-err", wPub)

	lr := &mockLogReader{rawCP: []byte("fake-cp")}
	feeder := NewWitnessFeeder(lr, []ExternalWitness{
		{Name: "bad-witness", URL: srv.URL, Verifier: wVerifier},
	}, 0)

	rawCP, _ := makeRawCP(t, "test-operator", 5)
	results := feeder.Feed(context.Background(), rawCP, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result")
	}
	if results[0].Err == nil {
		t.Error("expected error from bad witness, got nil")
	}
}

func TestWitnessFeeder_InvalidURL(t *testing.T) {
	wPub, _, _ := ed25519.GenerateKey(rand.Reader)
	wVerifier, _ := NewNoteVerifier("mock-witness-bad", wPub)

	lr := &mockLogReader{}
	feeder := NewWitnessFeeder(lr, []ExternalWitness{
		{Name: "bad-url", URL: "://not-a-url", Verifier: wVerifier},
	}, 0)

	rawCP, _ := makeRawCP(t, "test-operator", 5)
	results := feeder.Feed(context.Background(), rawCP, nil)
	if len(results) != 1 || results[0].Err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestWitnessFeeder_MultipleWitnesses(t *testing.T) {
	makeServer := func(code int) *httptest.Server {
		mock := &mockWitnessServer{
			respond: func(w http.ResponseWriter, _ []byte) {
				if code == http.StatusOK {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("cosigned"))
				} else {
					http.Error(w, "fail", code)
				}
			},
		}
		return httptest.NewServer(mock)
	}

	s1 := makeServer(http.StatusOK)
	defer s1.Close()
	s2 := makeServer(http.StatusInternalServerError)
	defer s2.Close()
	s3 := makeServer(http.StatusOK)
	defer s3.Close()

	newW := func(name, u string) ExternalWitness {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		v, _ := NewNoteVerifier(name, pub)
		return ExternalWitness{Name: name, URL: u, Verifier: v}
	}

	rawCP, _ := makeRawCP(t, "test-operator", 5)
	lr := &mockLogReader{rawCP: rawCP}
	feeder := NewWitnessFeeder(lr, []ExternalWitness{
		newW("w1", s1.URL),
		newW("w2", s2.URL),
		newW("w3", s3.URL),
	}, 0)

	results := feeder.Feed(context.Background(), rawCP, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	var errCount int
	for _, r := range results {
		if r.Err != nil {
			errCount++
		}
	}
	if errCount != 1 {
		t.Errorf("expected 1 error, got %d", errCount)
	}
}

func TestWitnessFeeder_Run_StopsOnCancel(t *testing.T) {
	rawCP, _ := makeRawCP(t, "test-operator", 5)
	mock := &mockWitnessServer{
		respond: func(w http.ResponseWriter, _ []byte) {
			w.WriteHeader(http.StatusOK)
			w.Write(rawCP)
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	wPub, _, _ := ed25519.GenerateKey(rand.Reader)
	wVerifier, _ := NewNoteVerifier("run-witness", wPub)

	lr := &mockLogReader{rawCP: rawCP}
	feeder := NewWitnessFeeder(lr, []ExternalWitness{
		{Name: "run-w", URL: srv.URL, Verifier: wVerifier},
	}, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		feeder.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("Run did not stop after context cancellation")
	}
}

func TestWitnessFeeder_ConsistencyProofUsedOnSecondFeed(t *testing.T) {
	rawCP, _ := makeRawCP(t, "test-operator", 5)

	var receivedBodies []string
	mock := &mockWitnessServer{
		respond: func(w http.ResponseWriter, body []byte) {
			receivedBodies = append(receivedBodies, string(body))
			w.WriteHeader(http.StatusOK)
			w.Write(rawCP)
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	wPub, _, _ := ed25519.GenerateKey(rand.Reader)
	wVerifier, _ := NewNoteVerifier("proof-witness", wPub)

	// Feed twice — second call should include "old <size>" in request body
	lr := &mockLogReader{rawCP: rawCP}
	feeder := NewWitnessFeeder(lr, []ExternalWitness{
		{Name: "proof-w", URL: srv.URL, Verifier: wVerifier},
	}, 0)

	feeder.Feed(context.Background(), rawCP, nil)
	feeder.Feed(context.Background(), rawCP, nil)

	if len(receivedBodies) < 2 {
		t.Fatalf("expected 2 requests, got %d", len(receivedBodies))
	}
	// Second request should start with "old 5" (from cached tree size)
	if !strings.HasPrefix(receivedBodies[1], "old 5") {
		t.Errorf("second request should start with 'old 5', got: %q", receivedBodies[1][:min(20, len(receivedBodies[1]))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
