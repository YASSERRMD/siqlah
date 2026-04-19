package provider

import (
	"encoding/json"
	"fmt"

	"github.com/yasserrmd/siqlah/pkg/vur"
)

// AnthropicAdapter parses Anthropic messages API response bodies.
type AnthropicAdapter struct{}

type anthropicResponse struct {
	ID    string `json:"id"`
	Usage struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func (AnthropicAdapter) Name() string        { return "anthropic" }
func (AnthropicAdapter) TokenizerID() string { return "cl100k_base" }

func (AnthropicAdapter) ParseUsage(body []byte) (*vur.ProviderUsage, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("anthropic parse: %w", err)
	}
	return &vur.ProviderUsage{
		InputTokens:      resp.Usage.InputTokens,
		OutputTokens:     resp.Usage.OutputTokens,
		CacheWriteTokens: resp.Usage.CacheCreationInputTokens,
		CacheReadTokens:  resp.Usage.CacheReadInputTokens,
		RequestID:        resp.ID,
	}, nil
}
