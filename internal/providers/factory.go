package providers

import (
	"strings"

	"github.com/kr0nicas/picobot/internal/config"
)

// NewProviderFromConfig creates a provider based on the configuration.
func NewProviderFromConfig(cfg config.Config) LLMProvider {
	model := cfg.Agents.Defaults.Model

	// If it's a Claude model and we have an Anthropic key, use the native provider.
	// (Note: AnthropicProvider implementation pending in anthropic.go)
	maxTokens := cfg.Agents.Defaults.MaxTokens
	timeout := cfg.Agents.Defaults.RequestTimeoutS

	if strings.HasPrefix(model, "claude-") && cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.APIKey != "" {
		return NewAnthropicProvider(
			cfg.Providers.Anthropic.APIKey,
			cfg.Providers.Anthropic.APIBase,
			timeout,
			maxTokens,
		)
	}

	// Default to OpenAI-compatible provider (works for GPT, Gemini, Grok, etc.)
	if cfg.Providers.OpenAI != nil && cfg.Providers.OpenAI.APIKey != "" {
		return NewOpenAIProvider(
			cfg.Providers.OpenAI.APIKey,
			cfg.Providers.OpenAI.APIBase,
			timeout,
			maxTokens,
		)
	}

	// Fallback to Anthropic if that's all we have and it wasn't caught by the model prefix
	if cfg.Providers.Anthropic != nil && cfg.Providers.Anthropic.APIKey != "" {
		return NewAnthropicProvider(
			cfg.Providers.Anthropic.APIKey,
			cfg.Providers.Anthropic.APIBase,
			timeout,
			maxTokens,
		)
	}

	return NewStubProvider()
}
