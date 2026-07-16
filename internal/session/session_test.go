package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "session.json") // sub dir does not exist yet
	in := Session{Selected: "com.a", TextPattern: "web", DomainScope: 2, SortKey: 1, SortDesc: true, ListScroll: 4, ActiveTab: 2}
	if err := Save(p, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := Load(p)
	if got != in {
		t.Fatalf("round trip: %+v != %+v", got, in)
	}
}

func TestLoadMissingIsZero(t *testing.T) {
	got := Load(filepath.Join(t.TempDir(), "nope.json"))
	if got != (Session{}) {
		t.Fatalf("missing → zero, got %+v", got)
	}
}

func TestLoadCorruptIsZero(t *testing.T) {
	p := filepath.Join(t.TempDir(), "session.json")
	os.WriteFile(p, []byte("{not json"), 0o644)
	if got := Load(p); got != (Session{}) {
		t.Fatalf("corrupt → zero, got %+v", got)
	}
}
