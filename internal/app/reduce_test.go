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

func TestSelectServiceResetsDetail(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1), svc("com.b", launchctl.GUIDomain(501), 2)), s)
	s.LogRing = []LogLine{{Stream: "out", Text: "old"}}
	s = Reduce(SelectService{Label: "com.b"}, s)
	if s.Selected != "com.b" || s.Detail.LoadState != DetailLoading || len(s.LogRing) != 0 {
		t.Fatalf("select should reset detail+log: %+v", s)
	}
	if s.Gone {
		t.Fatal("selecting a present service clears gone")
	}
}

func TestMoveSelectionClamps(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0), svc("com.b", launchctl.GUIDomain(501), 0)), s)
	// first scan selected com.a
	s = Reduce(MoveSelection{Delta: -1}, s) // already top, stays
	if s.Selected != "com.a" {
		t.Fatalf("clamp top: %q", s.Selected)
	}
	s = Reduce(MoveSelection{ToBottom: true}, s)
	if s.Selected != "com.b" {
		t.Fatalf("to bottom: %q", s.Selected)
	}
}

func TestFocusAndTab(t *testing.T) {
	s := NewState(501)
	s = Reduce(FocusPanel{}, s)
	if s.Focus != FocusDetail {
		t.Fatal("focus toggle")
	}
	s = Reduce(SetTab{Tab: TabRaw}, s)
	if s.ActiveTab != TabRaw {
		t.Fatal("set tab")
	}
}

func TestFilterEditCommit(t *testing.T) {
	s := NewState(501)
	s.Filters.TextPattern = "old"
	s = Reduce(OpenFilter{}, s)
	if !s.FilterEditing || s.FilterBuffer != "old" {
		t.Fatalf("open seeds buffer: %+v", s)
	}
	s = Reduce(SetFilterBuffer{Buffer: "new"}, s)
	s = Reduce(CommitFilter{}, s)
	if s.FilterEditing || s.Filters.TextPattern != "new" {
		t.Fatalf("commit: %+v", s)
	}
}

func TestFilterCancelRestores(t *testing.T) {
	s := NewState(501)
	s.Filters.TextPattern = "keep"
	s = Reduce(OpenFilter{}, s)
	s = Reduce(SetFilterBuffer{Buffer: "typed"}, s)
	s = Reduce(CancelFilter{}, s)
	if s.FilterEditing || s.Filters.TextPattern != "keep" {
		t.Fatalf("cancel: %+v", s)
	}
}

func TestCycleDomainScope(t *testing.T) {
	s := NewState(501) // ScopeAll
	s = Reduce(CycleDomainScope{}, s)
	if s.Filters.DomainScope != ScopeUser { // all → user → system → all
		t.Fatalf("cycle from all: %v", s.Filters.DomainScope)
	}
}

func TestSetSort(t *testing.T) {
	s := NewState(501) // SortLabel asc
	s = Reduce(SetSort{}, s)
	if s.SortKey != SortStatus {
		t.Fatalf("cycle key: %v", s.SortKey)
	}
	s = Reduce(SetSort{ToggleDir: true}, s)
	if !s.SortDesc {
		t.Fatal("toggle dir")
	}
}
