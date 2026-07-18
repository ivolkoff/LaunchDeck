package app

import (
	"testing"

	"github.com/ivolkoff/launchdeck/internal/launchctl"
)

func svc(label string, dom launchctl.Domain, pid int) launchctl.Service {
	s := launchctl.Service{Label: label, Domain: dom}
	if pid > 0 {
		s.PID, s.HasPID = pid, true
	}
	return s
}

func TestApplyFilterTextCaseInsensitive(t *testing.T) {
	in := []launchctl.Service{
		svc("com.example.Foo", launchctl.GUIDomain(501), 0),
		svc("com.other.bar", launchctl.GUIDomain(501), 0),
	}
	out := applyFilter(in, Filters{DomainScope: ScopeAll, TextPattern: "foo"}, 501)
	if len(out) != 1 || out[0].Label != "com.example.Foo" {
		t.Fatalf("text filter: %#v", out)
	}
	// empty pattern → all
	if len(applyFilter(in, Filters{DomainScope: ScopeAll}, 501)) != 2 {
		t.Fatal("empty pattern should match all")
	}
}

func TestApplyFilterDomainScope(t *testing.T) {
	in := []launchctl.Service{
		svc("a", launchctl.GUIDomain(501), 0),
		svc("b", launchctl.SystemDomain(), 0),
	}
	if got := applyFilter(in, Filters{DomainScope: ScopeUser}, 501); len(got) != 1 || got[0].Label != "a" {
		t.Fatalf("user scope: %#v", got)
	}
	if got := applyFilter(in, Filters{DomainScope: ScopeSystem}, 501); len(got) != 1 || got[0].Label != "b" {
		t.Fatalf("system scope: %#v", got)
	}
}

func TestApplySortPIDNullsLast(t *testing.T) {
	in := []launchctl.Service{
		svc("z", launchctl.GUIDomain(501), 0), // stopped, null PID
		svc("a", launchctl.GUIDomain(501), 30),
		svc("b", launchctl.GUIDomain(501), 10),
	}
	out := applySort(in, SortPID, false)
	if out[0].Label != "b" || out[1].Label != "a" || out[2].Label != "z" {
		t.Fatalf("pid asc, null last: %v %v %v", out[0].Label, out[1].Label, out[2].Label)
	}
}

func TestApplySortLabelSecondaryTieBreak(t *testing.T) {
	// Two running services tie on status; secondary key = label ascending.
	in := []launchctl.Service{
		svc("Beta", launchctl.GUIDomain(501), 2),
		svc("alpha", launchctl.GUIDomain(501), 3),
	}
	out := applySort(in, SortStatus, false)
	if out[0].Label != "alpha" || out[1].Label != "Beta" {
		t.Fatalf("status tie → label secondary: %v", out)
	}
}
