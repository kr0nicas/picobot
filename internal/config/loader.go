package config

import (
	"encoding/json"
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

	// Environment variable overrides for security and docker flexibility
	// LLM API Key (support multiple naming conventions)
	llmKey := os.Getenv("PICOBOT_LLM_API_KEY")
	if llmKey == "" {
		llmKey = os.Getenv("PICOBOT_OPENAI_API_KEY")
	}
	if llmKey == "" {
		llmKey = os.Getenv("OPENAI_API_KEY")
	}

	if llmKey != "" {
		if cfg.Providers.OpenAI == nil {
			cfg.Providers.OpenAI = &ProviderConfig{}
		}
		cfg.Providers.OpenAI.APIKey = llmKey
	}

	// LLM API Base (for Google Gemini or local Ollama)
	llmBase := os.Getenv("PICOBOT_LLM_API_BASE")
	if llmBase == "" {
		llmBase = os.Getenv("OPENAI_API_BASE")
	}
	if llmBase != "" {
		if cfg.Providers.OpenAI == nil {
			cfg.Providers.OpenAI = &ProviderConfig{}
		}
		cfg.Providers.OpenAI.APIBase = llmBase
	}

	// LLM Model
	if model := os.Getenv("PICOBOT_LLM_MODEL"); model != "" {
		cfg.Agents.Defaults.Model = model
	} else if model := os.Getenv("PICOBOT_MODEL"); model != "" {
		cfg.Agents.Defaults.Model = model
	}

	// Telegram
	if token := os.Getenv("PICOBOT_TELEGRAM_TOKEN"); token != "" {
		cfg.Channels.Telegram.Token = token
	} else if token := os.Getenv("PICOBOT_GATEWAY_TELEGRAM_TOKEN"); token != "" {
		cfg.Channels.Telegram.Token = token
	}

	if allowed := os.Getenv("PICOBOT_TELEGRAM_ALLOWED_USERS"); allowed != "" {
		cfg.Channels.Telegram.AllowFrom = strings.Split(allowed, ",")
	} else if allowed := os.Getenv("TELEGRAM_ALLOW_FROM"); allowed != "" {
		cfg.Channels.Telegram.AllowFrom = strings.Split(allowed, ",")
	}

	return cfg, nil
}
