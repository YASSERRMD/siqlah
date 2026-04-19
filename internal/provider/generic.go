package provider

import (
	"encoding/json"
	"fmt"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// GenericAdapter handles any OpenAI-compatible endpoint (Ollama, vLLM, llama.cpp, LiteLLM).
type GenericAdapter struct{}

type genericResponse struct {
	ID    string `json:"id"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		TotalTokens      int64 `json:"total_tokens"`
	} `json:"usage"`
}

func (GenericAdapter) Name() string        { return "generic" }
func (GenericAdapter) TokenizerID() string { return "cl100k_base" }

func (GenericAdapter) ParseUsage(body []byte) (*vur.ProviderUsage, error) {
	var resp genericResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("generic parse: %w", err)
	}
	return &vur.ProviderUsage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		RequestID:    resp.ID,
	}, nil
}
