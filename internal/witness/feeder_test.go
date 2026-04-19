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

	"golang.org/x/mod/sumdb/note"
)

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

func TestWitnessFeeder_FeedOne_Success(t *testing.T) {
	// Stand up a mock external witness that echoes back a cosigned note.
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	operatorSigner, err := NewNoteSigner("test-operator", priv)
	if err != nil {
		t.Fatal(err)
	}

	body := "test-operator/log\n5\nabcdef==\n\n"
	rawCP, err := note.Sign(&note.Note{Text: body}, operatorSigner)
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockWitnessServer{
		respond: func(w http.ResponseWriter, _ []byte) {
			w.WriteHeader(http.StatusOK)
			w.Write(rawCP) // echo back the checkpoint as "cosigned"
		},
	}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	wPub, _, _ := ed25519.GenerateKey(rand.Reader)
	wVerifier, err := NewNoteVerifier("mock-witness-1", wPub)
	if err != nil {
		t.Fatal(err)
	}

	feeder := NewWitnessFeeder([]ExternalWitness{
		{Name: "mock-witness", URL: srv.URL, Verifier: wVerifier},
	})

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

	feeder := NewWitnessFeeder([]ExternalWitness{
		{Name: "bad-witness", URL: srv.URL, Verifier: wVerifier},
	})

	results := feeder.Feed(context.Background(), []byte("fake-cp"), nil)
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

	feeder := NewWitnessFeeder([]ExternalWitness{
		{Name: "bad-url", URL: "://not-a-url", Verifier: wVerifier},
	})

	results := feeder.Feed(context.Background(), []byte("cp"), nil)
	if len(results) != 1 || results[0].Err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestWitnessFeeder_MultipleWitnesses(t *testing.T) {
	successes := 0
	makeServer := func(code int) *httptest.Server {
		mock := &mockWitnessServer{
			respond: func(w http.ResponseWriter, _ []byte) {
				if code == http.StatusOK {
					successes++
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

	newW := func(name, url string) ExternalWitness {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		v, _ := NewNoteVerifier(name, pub)
		return ExternalWitness{Name: name, URL: url, Verifier: v}
	}

	feeder := NewWitnessFeeder([]ExternalWitness{
		newW("w1", s1.URL),
		newW("w2", s2.URL),
		newW("w3", s3.URL),
	})

	results := feeder.Feed(context.Background(), []byte("raw-cp"), nil)
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
