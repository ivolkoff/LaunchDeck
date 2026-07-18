package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPresent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{"lang":"ru"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(p).Lang; got != "ru" {
		t.Errorf("Lang = %q, want ru", got)
	}
}

func TestLoadAbsent(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nope.json")
	if got := Load(p).Lang; got != "" {
		t.Errorf("absent → Lang = %q, want empty", got)
	}
}

func TestLoadCorrupt(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(`{ not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Load(p).Lang; got != "" {
		t.Errorf("corrupt → Lang = %q, want empty", got)
	}
}
