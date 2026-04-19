package provider_test

import (
	"testing"

	"github.com/yasserrmd/siqlah/internal/provider"
)

const openAIResponse = `{
  "id": "chatcmpl-abc123",
  "usage": {
    "prompt_tokens": 150,
    "completion_tokens": 45,
    "completion_tokens_details": {"reasoning_tokens": 10}
  }
}`

const openAIZeroResponse = `{"id":"","usage":{}}`

const anthropicResponse = `{
  "id": "msg-xyz",
  "usage": {
    "input_tokens": 200,
    "output_tokens": 80,
    "cache_creation_input_tokens": 50,
    "cache_read_input_tokens": 30
  }
}`

const genericResponse = `{
  "id": "req-001",
  "usage": {"prompt_tokens": 60, "completion_tokens": 20, "total_tokens": 80}
}`

func TestOpenAIAdapter(t *testing.T) {
	a := provider.OpenAIAdapter{}
	u, err := a.ParseUsage([]byte(openAIResponse))
	if err != nil {
		t.Fatal(err)
	}
	if u.InputTokens != 150 {
		t.Errorf("input: got %d, want 150", u.InputTokens)
	}
	if u.OutputTokens != 45 {
		t.Errorf("output: got %d, want 45", u.OutputTokens)
	}
	if u.ReasoningTokens != 10 {
		t.Errorf("reasoning: got %d, want 10", u.ReasoningTokens)
	}
	if u.RequestID != "chatcmpl-abc123" {
		t.Errorf("request ID: got %s", u.RequestID)
	}
}

func TestOpenAIZeroTokens(t *testing.T) {
	u, err := provider.OpenAIAdapter{}.ParseUsage([]byte(openAIZeroResponse))
	if err != nil {
		t.Fatal(err)
	}
	if u.InputTokens != 0 || u.OutputTokens != 0 {
		t.Errorf("expected zero tokens")
	}
}

func TestAnthropicAdapter(t *testing.T) {
	a := provider.AnthropicAdapter{}
	u, err := a.ParseUsage([]byte(anthropicResponse))
	if err != nil {
		t.Fatal(err)
	}
	if u.InputTokens != 200 {
		t.Errorf("input: got %d, want 200", u.InputTokens)
	}
	if u.OutputTokens != 80 {
		t.Errorf("output: got %d, want 80", u.OutputTokens)
	}
	if u.CacheWriteTokens != 50 {
		t.Errorf("cache_write: got %d, want 50", u.CacheWriteTokens)
	}
	if u.CacheReadTokens != 30 {
		t.Errorf("cache_read: got %d, want 30", u.CacheReadTokens)
	}
}

func TestGenericAdapter(t *testing.T) {
	u, err := provider.GenericAdapter{}.ParseUsage([]byte(genericResponse))
	if err != nil {
		t.Fatal(err)
	}
	if u.InputTokens != 60 || u.OutputTokens != 20 {
		t.Errorf("tokens mismatch")
	}
}

func TestRegistry(t *testing.T) {
	reg := provider.NewRegistry()
	for _, name := range []string{"openai", "anthropic", "generic"} {
		a, err := reg.Get(name)
		if err != nil {
			t.Errorf("get %s: %v", name, err)
			continue
		}
		if a.Name() != name {
			t.Errorf("name mismatch: got %s, want %s", a.Name(), name)
		}
	}
	if _, err := reg.Get("unknown"); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestBadJSON(t *testing.T) {
	oai := provider.OpenAIAdapter{}
	if _, err := oai.ParseUsage([]byte("not json")); err == nil {
		t.Error("openai: expected error for invalid JSON")
	}
	ant := provider.AnthropicAdapter{}
	if _, err := ant.ParseUsage([]byte("{bad")); err == nil {
		t.Error("anthropic: expected error for invalid JSON")
	}
	gen := provider.GenericAdapter{}
	if _, err := gen.ParseUsage([]byte("")); err == nil {
		t.Error("generic: expected error for empty body")
	}
}
