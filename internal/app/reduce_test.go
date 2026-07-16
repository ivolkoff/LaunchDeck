package app

import (
	"testing"

	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func loaded(svcs ...launchctl.Service) ServicesLoaded { return ServicesLoaded{Services: svcs} }

func TestFirstScanBindsPersistedSelection(t *testing.T) {
	s := NewState(501)
	s.Selected = "com.b" // as if restored from session
	s = Reduce(loaded(
		svc("com.a", launchctl.GUIDomain(501), 0),
		svc("com.b", launchctl.GUIDomain(501), 9),
	), s)
	if s.Selected != "com.b" || !s.FirstScanDone || !s.SelectionResolved {
		t.Fatalf("persisted selection not bound: %+v", s)
	}
}

func TestFirstScanFallsBackToFirstVisible(t *testing.T) {
	s := NewState(501)
	s.Selected = "com.missing"
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	if s.Selected != "com.a" {
		t.Fatalf("want first-visible fallback, got %q", s.Selected)
	}
}

func TestFirstScanEmptyVisibleClears(t *testing.T) {
	s := NewState(501)
	s.Selected = "com.a"
	s.Filters.TextPattern = "zzz" // matches nothing
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	if s.Selected != "" || !s.FirstScanDone {
		t.Fatalf("empty visible should clear selection: %+v", s)
	}
}

func TestLaterScanGoneThenRebind(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s) // first scan binds com.a
	s = Reduce(loaded(svc("com.b", launchctl.GUIDomain(501), 2)), s) // com.a vanished
	if !s.Gone || s.Selected != "com.a" {
		t.Fatalf("want (gone) com.a, got selected=%q gone=%v", s.Selected, s.Gone)
	}
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 3)), s) // reappears
	if s.Gone {
		t.Fatalf("com.a reappeared, should re-bind")
	}
}

func TestScanErrorKeepsPriorList(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s)
	s = Reduce(ServicesLoaded{Err: &launchctl.ScanError{Kind: launchctl.FailureGeneric, Stderr: "boom"}}, s)
	if len(s.Services) != 1 {
		t.Fatalf("scan error should keep prior list, got %d", len(s.Services))
	}
	if s.StatusMsg == "" {
		t.Fatal("scan error should set a status banner")
	}
}
