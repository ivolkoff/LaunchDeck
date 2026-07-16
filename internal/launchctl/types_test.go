package launchctl

import "testing"

func TestDomainString(t *testing.T) {
	if got := GUIDomain(501).String(); got != "gui/501" {
		t.Fatalf("gui: got %q", got)
	}
	if got := SystemDomain().String(); got != "system" {
		t.Fatalf("system: got %q", got)
	}
}

func TestDomainTarget(t *testing.T) {
	if got := GUIDomain(501).Target("com.example.a"); got != "gui/501/com.example.a" {
		t.Fatalf("gui target: got %q", got)
	}
	if got := SystemDomain().Target("com.example.a"); got != "system/com.example.a" {
		t.Fatalf("system target: got %q", got)
	}
}
