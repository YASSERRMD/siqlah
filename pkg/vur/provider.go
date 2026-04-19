package vur

// ProviderUsage holds token usage data extracted from a provider response.
type ProviderUsage struct {
	InputTokens      int64
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	RequestID        string
}

// ProviderAdapter parses provider-specific response bodies into a generic ProviderUsage.
type ProviderAdapter interface {
	Name() string
	ParseUsage(responseBody []byte) (*ProviderUsage, error)
	TokenizerID() string
}
