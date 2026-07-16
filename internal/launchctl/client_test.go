package launchctl

import (
	"context"
	"strings"
	"testing"
)

func TestClientScanDomain(t *testing.T) {
	c := newWith(func(_ context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		if name != "launchctl" || args[0] != "print" || args[1] != "gui/501" {
			t.Fatalf("unexpected argv: %s %v", name, args)
		}
		return []byte(guiScanFixture), nil, 0, nil
	})
	svcs, err := c.ScanDomain(context.Background(), GUIDomain(501))
	if err != nil || len(svcs) != 4 {
		t.Fatalf("scan: err=%v n=%d", err, len(svcs))
	}
}

func TestClientActionPermission(t *testing.T) {
	c := newWith(func(_ context.Context, _ string, _ ...string) ([]byte, []byte, int, error) {
		return nil, []byte("Operation not permitted"), 1, nil
	})
	out := c.Action(context.Background(), ActionStop, SystemDomain(), "com.x")
	if out.Kind != FailurePermission {
		t.Fatalf("want permission, got %v (stderr=%q)", out.Kind, out.Stderr)
	}
}

func TestClientActionArgv(t *testing.T) {
	var gotArgs []string
	c := newWith(func(_ context.Context, _ string, args ...string) ([]byte, []byte, int, error) {
		gotArgs = args
		return nil, nil, 0, nil
	})
	c.Action(context.Background(), ActionRestart, GUIDomain(501), "com.x")
	if strings.Join(gotArgs, " ") != "kickstart -k gui/501/com.x" {
		t.Fatalf("restart argv: %v", gotArgs)
	}
}
