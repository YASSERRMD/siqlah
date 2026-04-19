package provider

import (
	"fmt"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// Registry maps provider names to their adapters.
type Registry struct {
	adapters map[string]vur.ProviderAdapter
}

// NewRegistry returns a registry pre-populated with built-in adapters.
func NewRegistry() *Registry {
	r := &Registry{adapters: map[string]vur.ProviderAdapter{}}
	r.Register(OpenAIAdapter{})
	r.Register(AnthropicAdapter{})
	r.Register(GenericAdapter{})
	return r
}

// Register adds an adapter to the registry under its Name().
func (r *Registry) Register(a vur.ProviderAdapter) {
	r.adapters[a.Name()] = a
}

// Get returns the adapter for the given provider name.
func (r *Registry) Get(name string) (vur.ProviderAdapter, error) {
	a, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return a, nil
}
