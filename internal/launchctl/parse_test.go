package launchctl

import "testing"

const guiScanFixture = `gui/501 = {
	type = User
	handle = 501
	active count = 42
	services = {
		12345	0	com.example.running
		-	0	com.example.stopped
		-	78	com.example.crashed
		-	-	com.apple.never.ran
	}
}
`

func TestParseDomainScan(t *testing.T) {
	svcs, err := parseDomainScan(guiScanFixture, GUIDomain(501))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(svcs) != 4 {
		t.Fatalf("want 4 services, got %d", len(svcs))
	}
	run := svcs[0]
	if run.Label != "com.example.running" || !run.HasPID || run.PID != 12345 || run.LastExit != 0 {
		t.Fatalf("running row wrong: %+v", run)
	}
	if run.Domain.String() != "gui/501" {
		t.Fatalf("domain not stamped: %v", run.Domain)
	}
	stopped := svcs[1]
	if stopped.HasPID || stopped.PID != 0 || stopped.LastExit != 0 {
		t.Fatalf("stopped row wrong: %+v", stopped)
	}
	crashed := svcs[2]
	if crashed.HasPID || crashed.LastExit != 78 {
		t.Fatalf("crashed row wrong: %+v", crashed)
	}
}

func TestParseDomainScanNoServicesBlock(t *testing.T) {
	if _, err := parseDomainScan("gui/501 = {\n}\n", GUIDomain(501)); err == nil {
		t.Fatal("expected error when services block absent")
	}
}

const detailFixture = `com.example.running = {
	active count = 1
	path = /Users/me/Library/LaunchAgents/com.example.running.plist
	state = running
	program = /usr/local/bin/agent
	arguments = {
		/usr/local/bin/agent
		--serve
	}
	pid = 12345
	last exit code = 0
	stdout path = /tmp/agent.out
	stderr path = /tmp/agent.err
	disabled = false
}
`

func TestParseServiceDetail(t *testing.T) {
	base := Service{Label: "com.example.running", Domain: GUIDomain(501), PID: 12345, HasPID: true}
	d := parseServiceDetail(detailFixture, base)
	if d.Program != "/usr/local/bin/agent" {
		t.Fatalf("program: %q", d.Program)
	}
	if len(d.Args) != 2 || d.Args[1] != "--serve" {
		t.Fatalf("args: %#v", d.Args)
	}
	if d.PlistPath != "/Users/me/Library/LaunchAgents/com.example.running.plist" {
		t.Fatalf("plist: %q", d.PlistPath)
	}
	if d.StdoutPath != "/tmp/agent.out" || d.StderrPath != "/tmp/agent.err" {
		t.Fatalf("log paths: %q %q", d.StdoutPath, d.StderrPath)
	}
	if d.EnableState != Enabled {
		t.Fatalf("enableState: %v", d.EnableState)
	}
	if d.Raw != detailFixture {
		t.Fatal("raw not preserved verbatim")
	}
}

func TestParseServiceDetailDisabled(t *testing.T) {
	d := parseServiceDetail("x = {\n\tdisabled = true\n}\n", Service{Label: "x"})
	if d.EnableState != Disabled {
		t.Fatalf("want Disabled, got %v", d.EnableState)
	}
}

func TestClassifyFailure(t *testing.T) {
	cases := []struct {
		exit   int
		stderr string
		want   FailureKind
	}{
		{0, "anything", FailureGeneric},                       // exit 0 is never a failure
		{1, "Operation not permitted", FailurePermission},
		{1, "permission denied", FailurePermission},
		{1, "Bootstrap failed: 5: Input/output error", FailureGeneric},
		{1, "Could not find service (errno 1)", FailurePermission},
		{1, "failed (errno 13)", FailurePermission},
		{1, "failed (errno 12)", FailureGeneric},
		{1, "failed (errno 19)", FailureGeneric},
		{1, "failed (errno 100)", FailureGeneric},
		{1, "service is not privileged", FailurePermission},
	}
	for _, c := range cases {
		if got := ClassifyFailure(c.exit, c.stderr); got != c.want {
			t.Errorf("Classify(%d,%q) = %v, want %v", c.exit, c.stderr, got, c.want)
		}
	}
}
