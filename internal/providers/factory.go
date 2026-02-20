package providers

import "github.com/kr0nicas/picobot/internal/config"

// NewProviderFromConfig creates a provider based on the configuration.
// Simple rules (v0):
//   - if OpenAI API key present -> OpenAI
//   - else fallback to stub
func NewProviderFromConfig(cfg config.Config) LLMProvider {
	if cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.APIKey != "" {
		return NewOpenAIProvider(
			cfg.Providers.OpenAI.APIKey,
			cfg.Providers.OpenAI.APIBase,
			cfg.Agents.Defaults.RequestTimeoutS,
		)
	}
	return NewStubProvider()
}
