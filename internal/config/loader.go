package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// LoadConfig loads config from ~/.picobot/config.json (or PICOBOT_HOME) if present,
// then overrides sensitive fields with environment variables if set.
func LoadConfig() (Config, error) {
	var path string
	if ph := os.Getenv("PICOBOT_HOME"); ph != "" {
		path = filepath.Join(ph, "config.json")
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		path = filepath.Join(home, ".picobot", "config.json")
	}

	var cfg Config
	f, err := os.Open(path)
	if err == nil {
		dec := json.NewDecoder(f)
		if err := dec.Decode(&cfg); err != nil {
			f.Close()
			return Config{}, err
		}
		f.Close()
	}

	// Environment variable overrides for security and docker flexibility (Supports GIO_ and PICOBOT_ prefixes)
	// LLM API Key
	llmKey := strings.TrimSpace(os.Getenv("GIO_LLM_API_KEY"))
	if llmKey == "" {
		llmKey = strings.TrimSpace(os.Getenv("PICOBOT_LLM_API_KEY"))
	}
	if llmKey == "" {
		llmKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}

	if llmKey != "" {
		if strings.HasSuffix(llmKey, "...") {
			log.Printf("WARNING: LLM API Key seems to be truncated (ends with '...')")
		}
		if cfg.Providers.OpenAI == nil {
			cfg.Providers.OpenAI = &ProviderConfig{}
		}
		cfg.Providers.OpenAI.APIKey = llmKey
	}

	// Anthropic API Key
	anthropicKey := strings.TrimSpace(os.Getenv("GIO_ANTHROPIC_API_KEY"))
	if anthropicKey == "" {
		anthropicKey = strings.TrimSpace(os.Getenv("PICOBOT_ANTHROPIC_API_KEY"))
	}
	if anthropicKey == "" {
		anthropicKey = strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	}
	if anthropicKey != "" {
		if cfg.Providers.Anthropic == nil {
			cfg.Providers.Anthropic = &ProviderConfig{}
		}
		cfg.Providers.Anthropic.APIKey = anthropicKey
	}

	// LLM API Base (for Google Gemini or local Ollama)
	llmBase := strings.TrimSpace(os.Getenv("GIO_LLM_API_BASE"))
	if llmBase == "" {
		llmBase = strings.TrimSpace(os.Getenv("PICOBOT_LLM_API_BASE"))
	}
	if llmBase == "" {
		llmBase = strings.TrimSpace(os.Getenv("OPENAI_API_BASE"))
	}
	if llmBase != "" {
		if cfg.Providers.OpenAI == nil {
			cfg.Providers.OpenAI = &ProviderConfig{}
		}
		cfg.Providers.OpenAI.APIBase = strings.TrimRight(llmBase, "/")
	}

	// Anthropic API Base
	anthropicBase := strings.TrimSpace(os.Getenv("GIO_ANTHROPIC_API_BASE"))
	if anthropicBase == "" {
		anthropicBase = strings.TrimSpace(os.Getenv("ANTHROPIC_API_BASE"))
	}
	if anthropicBase != "" {
		if cfg.Providers.Anthropic == nil {
			cfg.Providers.Anthropic = &ProviderConfig{}
		}
		cfg.Providers.Anthropic.APIBase = strings.TrimRight(anthropicBase, "/")
	}

	// LLM Model
	model := strings.TrimSpace(os.Getenv("GIO_LLM_MODEL"))
	if model == "" {
		model = strings.TrimSpace(os.Getenv("PICOBOT_LLM_MODEL"))
	}
	if model == "" {
		model = strings.TrimSpace(os.Getenv("PICOBOT_MODEL"))
	}
	if model != "" {
		cfg.Agents.Defaults.Model = model
	}

	// Telegram
	token := strings.TrimSpace(os.Getenv("GIO_TELEGRAM_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("PICOBOT_TELEGRAM_TOKEN"))
	}
	if token == "" {
		token = strings.TrimSpace(os.Getenv("PICOBOT_GATEWAY_TELEGRAM_TOKEN"))
	}
	if token != "" {
		cfg.Channels.Telegram.Token = token
		cfg.Channels.Telegram.Enabled = true // Auto-enable if token is provided via ENV
	}

	// Allowed Users
	allowed := strings.TrimSpace(os.Getenv("GIO_TELEGRAM_ALLOWED_USERS"))
	if allowed == "" {
		allowed = strings.TrimSpace(os.Getenv("PICOBOT_TELEGRAM_ALLOWED_USERS"))
	}
	if allowed == "" {
		allowed = strings.TrimSpace(os.Getenv("TELEGRAM_ALLOW_FROM"))
	}
	if allowed != "" {
		cfg.Channels.Telegram.AllowFrom = strings.Split(allowed, ",")
	}

	return cfg, nil
}
