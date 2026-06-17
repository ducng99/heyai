package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	APIKey   string     `json:"api_key"`
	BaseURL  string     `json:"base_url"`
	Model    string     `json:"model"`
	MaxTurns int        `json:"max_turns"`
	Bash     BashConfig `json:"bash"`
}

type BashConfig struct {
	TimeoutMS                int  `json:"timeout_ms"`
	AllowRiskyWithoutConfirm bool `json:"allow_risky_without_confirm"`
	MaxOutputBytes           int  `json:"max_output_bytes"`
}

func defaultConfig() Config {
	return Config{
		BaseURL:  "https://api.openai.com",
		Model:    "gpt-4o-mini",
		MaxTurns: 8,
		Bash: BashConfig{
			TimeoutMS:      30000,
			MaxOutputBytes: 20000,
		},
	}
}

func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "heyai", "config.json"), nil
}

func LoadConfig() (Config, error) {
	cfg := defaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("config not found at %s; run heyai --init", path)
		}
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	d := defaultConfig()
	if cfg.BaseURL == "" {
		cfg.BaseURL = d.BaseURL
	}
	if cfg.Model == "" {
		cfg.Model = d.Model
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = d.MaxTurns
	}
	if cfg.Bash.TimeoutMS == 0 {
		cfg.Bash.TimeoutMS = d.Bash.TimeoutMS
	}
	if cfg.Bash.MaxOutputBytes == 0 {
		cfg.Bash.MaxOutputBytes = d.Bash.MaxOutputBytes
	}
}

func initConfig() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config already exists: %s", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	cfg := defaultConfig()
	cfg.APIKey = "sk-..."
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0600); err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
