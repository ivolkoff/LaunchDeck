// Package session persists the UI session (selection, filters, sort, scroll, tab)
// so relaunching restores the view.
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Session struct {
	Selected    string `json:"selected"`
	TextPattern string `json:"text_pattern"`
	DomainScope int    `json:"domain_scope"`
	SortKey     int    `json:"sort_key"`
	SortDesc    bool   `json:"sort_desc"`
	ListScroll  int    `json:"list_scroll"`
	ActiveTab   int    `json:"active_tab"`
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "launchdeck", "session.json"), nil
}

// Load never errors: missing or corrupt file → zero Session.
func Load(path string) Session {
	var s Session
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}
	}
	if json.Unmarshal(data, &s) != nil {
		return Session{}
	}
	return s
}

// Save writes atomically (temp file + rename) after ensuring the dir exists.
func Save(path string, s Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "session-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
