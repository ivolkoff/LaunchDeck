package app

import (
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
