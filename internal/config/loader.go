package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadConfig loads config from ~/.picobot/config.json if present, then overrides
// sensitive fields with environment variables if set.
func LoadConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	path := filepath.Join(home, ".picobot", "config.json")
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

	// Environment variable overrides for security
	if key := os.Getenv("PICOBOT_OPENAI_API_KEY"); key != "" {
		if cfg.Providers.OpenAI == nil {
			cfg.Providers.OpenAI = &ProviderConfig{}
		}
		cfg.Providers.OpenAI.APIKey = key
	}
	if token := os.Getenv("PICOBOT_TELEGRAM_TOKEN"); token != "" {
		cfg.Channels.Telegram.Token = token
	}

	return cfg, nil
}
