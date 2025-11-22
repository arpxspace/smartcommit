package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ProviderType string

const (
	ProviderOpenAI ProviderType = "openai"
	ProviderOllama ProviderType = "ollama"
)

type Config struct {
	Provider     ProviderType `json:"provider"`
	OpenAIAPIKey string       `json:"openai_api_key"`
	OllamaModel  string       `json:"ollama_model"`
	OllamaURL    string       `json:"ollama_url"`
}

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".config", "smartcommit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// Try to load from file
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	// Fallback to defaults / env vars for backward compatibility or first run
	cfg := &Config{
		Provider:     ProviderOpenAI,
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OllamaModel:  "llama3",
		OllamaURL:    "http://localhost:11434",
	}

	return cfg, nil
}

func (c *Config) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
