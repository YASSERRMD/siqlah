package provider

import (
	"encoding/json"
	"fmt"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// OpenAIAdapter parses OpenAI chat completion response bodies.
type OpenAIAdapter struct{}

type openAIResponse struct {
	ID    string `json:"id"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
		CompletionTokensDetails *struct {
			ReasoningTokens int64 `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
}

func (OpenAIAdapter) Name() string        { return "openai" }
func (OpenAIAdapter) TokenizerID() string { return "cl100k_base" }

func (OpenAIAdapter) ParseUsage(body []byte) (*vur.ProviderUsage, error) {
	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("openai parse: %w", err)
	}
	u := &vur.ProviderUsage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
		RequestID:    resp.ID,
	}
	if resp.Usage.CompletionTokensDetails != nil {
		u.ReasoningTokens = resp.Usage.CompletionTokensDetails.ReasoningTokens
	}
	return u, nil
}
