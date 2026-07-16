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
