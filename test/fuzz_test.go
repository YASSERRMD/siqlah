package test

import (
	"testing"

	"github.com/yasserrmd/siqlah/internal/provider"
)

func FuzzOpenAIAdapter(f *testing.F) {
	// Seed corpus with valid and edge-case inputs.
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"usage":{"prompt_tokens":100,"completion_tokens":50}}`))
	f.Add([]byte(`{"id":"req-1","usage":{"prompt_tokens":0,"completion_tokens":0}}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"usage":{"prompt_tokens":-1,"completion_tokens":999999999}}`))

	adapter := provider.OpenAIAdapter{}
	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic; error is acceptable.
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("OpenAI adapter panicked on input %q: %v", data, r)
			}
		}()
		_, _ = adapter.ParseUsage(data)
	})
}

func FuzzAnthropicAdapter(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"usage":{"input_tokens":100,"output_tokens":50}}`))
	f.Add([]byte(`{"usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":500}}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"usage":null}`))

	adapter := provider.AnthropicAdapter{}
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Anthropic adapter panicked on input %q: %v", data, r)
			}
		}()
		_, _ = adapter.ParseUsage(data)
	})
}

func FuzzGenericAdapter(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	f.Add([]byte(`not json`))

	adapter := provider.GenericAdapter{}
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Generic adapter panicked on input %q: %v", data, r)
			}
		}()
		_, _ = adapter.ParseUsage(data)
	})
}

// Non-fuzz sanity tests using the fuzz seed corpus.
func TestOpenAIAdapterNopanicSeeds(t *testing.T) {
	adapter := provider.OpenAIAdapter{}
	seeds := [][]byte{
		{},
		[]byte(`{}`),
		[]byte(`not json`),
		[]byte(`null`),
		[]byte(`{"usage":{"prompt_tokens":100,"completion_tokens":50}}`),
	}
	for _, seed := range seeds {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on seed %q: %v", seed, r)
				}
			}()
			_, _ = adapter.ParseUsage(seed)
		}()
	}
}

func TestAnthropicAdapterNopanicSeeds(t *testing.T) {
	adapter := provider.AnthropicAdapter{}
	seeds := [][]byte{
		{},
		[]byte(`{}`),
		[]byte(`not json`),
		[]byte(`{"usage":{"input_tokens":100,"output_tokens":50}}`),
	}
	for _, seed := range seeds {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on seed %q: %v", seed, r)
				}
			}()
			_, _ = adapter.ParseUsage(seed)
		}()
	}
}
