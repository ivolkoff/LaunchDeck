// Package config reads LaunchDeck's optional ~/.config/launchdeck/config.json.
// It mirrors the theme loader: a missing or malformed file yields a zero Config
// (never an error), so callers always get usable defaults.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the non-visual settings. Lang is "ru" | "en" | "" (absent →
// auto-detect from the environment).
type Config struct {
	Lang string `json:"lang"`
}

// Path returns ~/.config/launchdeck/config.json.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "launchdeck", "config.json"), nil
}

// Load reads the config file. A missing or corrupt file yields a zero Config
// (never an error).
func Load(path string) Config {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = json.Unmarshal(data, &c) // parse error keeps the zero Config
	return c
}
