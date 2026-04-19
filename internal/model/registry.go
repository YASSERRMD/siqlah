package model

import (
	"errors"
	"sync"
)

// Registry caches known model identities.
type Registry struct {
	mu      sync.RWMutex
	byName  map[string]*ModelIdentity
	byDigest map[string]*ModelIdentity
}

// NewRegistry returns an empty Registry pre-populated with well-known models.
func NewRegistry() *Registry {
	r := &Registry{
		byName:   make(map[string]*ModelIdentity),
		byDigest: make(map[string]*ModelIdentity),
	}
	r.loadWellKnown()
	return r
}

// Register adds or replaces the identity for a named model.
func (r *Registry) Register(modelName string, identity ModelIdentity) error {
	if modelName == "" {
		return errors.New("model name must not be empty")
	}
	identity.ModelName = modelName

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[modelName] = &identity
	if identity.ModelDigest != "" {
		r.byDigest[identity.ModelDigest] = &identity
	}
	return nil
}

// Lookup returns the identity for the given model name.
func (r *Registry) Lookup(modelName string) (*ModelIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byName[modelName]
	if !ok {
		return nil, nil
	}
	cp := *id
	return &cp, nil
}

// LookupByDigest returns the identity for the given model digest.
func (r *Registry) LookupByDigest(digest string) (*ModelIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byDigest[digest]
	if !ok {
		return nil, nil
	}
	cp := *id
	return &cp, nil
}

// wellKnown contains published model identifiers for popular models.
// These represent provider-attested identifiers without OMS signatures.
var wellKnown = []ModelIdentity{
	{ModelName: "gpt-4o", SignerIdentity: "https://openai.com", Verified: false},
	{ModelName: "gpt-4o-mini", SignerIdentity: "https://openai.com", Verified: false},
	{ModelName: "gpt-4-turbo", SignerIdentity: "https://openai.com", Verified: false},
	{ModelName: "claude-opus-4", SignerIdentity: "https://anthropic.com", Verified: false},
	{ModelName: "claude-sonnet-4-5", SignerIdentity: "https://anthropic.com", Verified: false},
	{ModelName: "claude-haiku-4-5", SignerIdentity: "https://anthropic.com", Verified: false},
	{ModelName: "meta-llama/Llama-3.1-8B", SignerIdentity: "https://huggingface.co/meta-llama", Verified: false},
	{ModelName: "meta-llama/Llama-3.1-70B", SignerIdentity: "https://huggingface.co/meta-llama", Verified: false},
	{ModelName: "meta-llama/Llama-3.1-405B", SignerIdentity: "https://huggingface.co/meta-llama", Verified: false},
	{ModelName: "google/gemini-1.5-pro", SignerIdentity: "https://ai.google.dev", Verified: false},
	{ModelName: "google/gemini-1.5-flash", SignerIdentity: "https://ai.google.dev", Verified: false},
	{ModelName: "mistralai/Mistral-7B-Instruct-v0.3", SignerIdentity: "https://mistral.ai", Verified: false},
	{ModelName: "mistralai/Mixtral-8x7B-Instruct-v0.1", SignerIdentity: "https://mistral.ai", Verified: false},
}

func (r *Registry) loadWellKnown() {
	for _, id := range wellKnown {
		cp := id
		r.byName[id.ModelName] = &cp
	}
}
