package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model,omitempty"`
}

var validKeys = map[string]bool{
	"provider": true,
	"api-key":  true,
	"model":    true,
}

func ValidKeys() []string {
	return []string{"provider", "api-key", "model"}
}

func IsValidKey(key string) bool {
	return validKeys[key]
}

func Path() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gca", "config.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gca", "config.json")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return os.WriteFile(p, data, 0o600)
}

func (c *Config) Get(key string) string {
	switch key {
	case "provider":
		return c.Provider
	case "api-key":
		return c.APIKey
	case "model":
		return c.Model
	default:
		return ""
	}
}

func (c *Config) Set(key, value string) error {
	if !IsValidKey(key) {
		return fmt.Errorf("unknown config key %q. Valid keys: %s", key, strings.Join(ValidKeys(), ", "))
	}
	switch key {
	case "provider":
		value = strings.ToLower(value)
		if value != "groq" && value != "cerebras" {
			return fmt.Errorf("invalid provider %q. Choose: groq, cerebras", value)
		}
		c.Provider = value
	case "api-key":
		c.APIKey = value
	case "model":
		c.Model = value
	}
	return Save(c)
}

func MaskKey(key string) string {
	if len(key) <= 6 {
		return "***"
	}
	return key[:3] + "..." + key[len(key)-3:]
}
