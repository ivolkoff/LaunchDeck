package app

import (
	"fmt"
	"testing"

	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

func TestDeriveLoadingPlaceholder(t *testing.T) {
	s := NewState(501) // no scan yet
	vm := Derive(s)
	if vm.List.Placeholder != "Loading services…" {
		t.Fatalf("loading placeholder: %q", vm.List.Placeholder)
	}
}

func TestDeriveNoMatching(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	s.Filters.TextPattern = "zzz"
	vm := Derive(s)
	if vm.List.Placeholder != "No matching services" {
		t.Fatalf("no-match placeholder: %q", vm.List.Placeholder)
	}
}

func TestDeriveNoSelection(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 0)), s)
	s.Selected = ""
	vm := Derive(s)
	if vm.Detail.Mode != "empty" {
		t.Fatalf("no selection → empty detail, got %q", vm.Detail.Mode)
	}
}

func TestDeriveGoneFrozen(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s) // binds com.a
	s.Detail = Detail{LoadState: DetailReady, Metadata: launchctl.ServiceDetail{
		Service: launchctl.Service{Label: "com.a", Domain: launchctl.GUIDomain(501)}, Program: "/bin/x"}}
	s = Reduce(loaded(svc("com.b", launchctl.GUIDomain(501), 2)), s) // com.a gone
	vm := Derive(s)
	if vm.Detail.Mode != "gone" || vm.Detail.Program != "/bin/x" {
		t.Fatalf("gone should freeze last-known metadata: %+v", vm.Detail)
	}
}

func TestDeriveRowsSortedAndSelected(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(
		svc("com.b", launchctl.GUIDomain(501), 2),
		svc("com.a", launchctl.GUIDomain(501), 1),
	), s) // first scan selects the first VISIBLE row = com.a (label sort)
	vm := Derive(s)
	if len(vm.List.Rows) != 2 || vm.List.Rows[0].Label != "com.a" {
		t.Fatalf("rows sorted by label: %#v", vm.List.Rows)
	}
	if vm.List.SelectedIdx != 0 {
		t.Fatalf("selected idx: %d", vm.List.SelectedIdx)
	}
}

func TestDeriveListWindowed(t *testing.T) {
	s := NewState(501)
	var svcs []launchctl.Service
	for i := 0; i < 30; i++ {
		svcs = append(svcs, svc(fmt.Sprintf("com.%02d", i), launchctl.GUIDomain(501), 0))
	}
	s = Reduce(loaded(svcs...), s)
	s.ListViewportH = 10
	s.Scroll.List = 5
	vm := Derive(s)
	if len(vm.List.Rows) != 10 {
		t.Fatalf("window size: got %d rows, want 10", len(vm.List.Rows))
	}
	if vm.List.Rows[0].Label != "com.05" {
		t.Fatalf("window start: got %q, want com.05", vm.List.Rows[0].Label)
	}
}

func TestDeriveLogNewestFirst(t *testing.T) {
	s := NewState(501)
	s = Reduce(loaded(svc("com.a", launchctl.GUIDomain(501), 1)), s)
	s.Selected = "com.a"
	s.Detail.Metadata.StdoutPath = "/tmp/a.out"
	s.LogRing = []LogLine{
		{Stream: "out", Text: "first"},
		{Stream: "out", Text: "second"},
		{Stream: "out", Text: "third"},
	}
	vm := Derive(s)
	got := vm.Detail.LogLines
	if len(got) != 3 || got[0] != "[out] third" || got[2] != "[out] first" {
		t.Fatalf("logs should be newest-first: %#v", got)
	}
}
