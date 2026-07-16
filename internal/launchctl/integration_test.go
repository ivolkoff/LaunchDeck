package launchctl

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestIntegrationScanReadOnly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c := New()
	svcs, err := c.ScanDomain(ctx, GUIDomain(os.Getuid()))
	if err != nil {
		t.Fatalf("gui scan: %v", err)
	}
	if len(svcs) == 0 {
		t.Skip("no gui services on this machine")
	}
	for _, s := range svcs {
		if s.Label == "" {
			t.Fatal("parsed a service with empty label")
		}
		if s.Domain.String() != GUIDomain(os.Getuid()).String() {
			t.Fatalf("domain not stamped: %v", s.Domain)
		}
	}
	// One detail fetch must parse without crashing.
	if _, err := c.Print(ctx, GUIDomain(os.Getuid()), svcs[0].Label); err != nil {
		t.Logf("print %s: %v (non-fatal — some services deny inspect)", svcs[0].Label, err)
	}
}

func TestIntegrationActionRoundTrip(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("LAUNCHDECK_INTEGRATION") != "1" {
		t.Skip("set LAUNCHDECK_INTEGRATION=1 on darwin to run")
	}
	uid := os.Getuid()
	dom := GUIDomain(uid)
	label := "com.launchdeck.itest." + itoa(os.Getpid())
	dir := t.TempDir()
	plist := filepath.Join(dir, label+".plist")
	body := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>` + label + `</string>
  <key>ProgramArguments</key><array><string>/bin/sh</string><string>-c</string><string>while true; do sleep 1; done</string></array>
</dict></plist>`
	if err := os.WriteFile(plist, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New()
	ctx := context.Background()

	t.Cleanup(func() {
		c.Action(ctx, ActionUnload, dom, label) // bootout, ignore error
		os.Remove(plist)
	})

	if out := c.Bootstrap(ctx, dom, plist); !out.OK() {
		t.Fatalf("bootstrap: %+v", out)
	}
	if out := c.Action(ctx, ActionStart, dom, label); !out.OK() {
		t.Fatalf("kickstart: %+v", out)
	}
	// Give launchd a moment, then confirm a PID via print.
	time.Sleep(500 * time.Millisecond)
	d, err := c.Print(ctx, dom, label)
	if err != nil {
		t.Fatalf("print after start: %v", err)
	}
	if !d.HasPID {
		t.Fatalf("expected a running PID after kickstart, got %+v", d.Service)
	}
	if out := c.Action(ctx, ActionStop, dom, label); !out.OK() {
		t.Fatalf("kill: %+v", out)
	}
	if out := c.Action(ctx, ActionUnload, dom, label); !out.OK() {
		t.Fatalf("bootout: %+v", out)
	}
}

func itoa(n int) string { // avoid importing strconv just for the label
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
