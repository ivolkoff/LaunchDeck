package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567" // 40 chars
	cases := []struct {
		name                  string
		version, mainVer, rev string
		modified, hasInfo     bool
		want                  string
	}{
		{"release", "v1.2.3", "", "", false, false, "launchdeck v1.2.3"},
		{"module install", "dev", "v1.2.3", "", false, true, "launchdeck v1.2.3"},
		{"dev vcs clean", "dev", "", sha, false, true, "launchdeck dev (0123456789ab)"},
		{"dev vcs dirty", "dev", "", sha, true, true, "launchdeck dev (0123456789ab-dirty)"},
		{"dev short rev", "dev", "", "abc", false, true, "launchdeck dev (abc)"},
		{"dev devel no rev", "dev", "(devel)", "", false, true, "launchdeck dev"},
		{"dev no build info", "dev", "", "", false, false, "launchdeck dev"},
	}
	for _, c := range cases {
		if got := buildVersion(c.version, c.mainVer, c.rev, c.modified, c.hasInfo); got != c.want {
			t.Errorf("%s: buildVersion = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHelpText(t *testing.T) {
	h := helpText()
	for _, want := range []string{
		"Usage: launchdeck [flags]",
		"~/.config/launchdeck/session.json",
		"~/.config/launchdeck/theme.json",
		"Press ?",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("helpText missing %q", want)
		}
	}
	if !strings.Contains(h, "config.json") {
		t.Errorf("help text should mention config.json")
	}
}

func TestCrashMessage(t *testing.T) {
	const url = "please report: https://github.com/volkoffskij/launchdeck/issues"

	got := crashMessage("boom", "launchdeck v1.2.3")
	want := "launchdeck v1.2.3 crashed: boom\n" + url
	if got != want {
		t.Errorf("string value:\n got %q\nwant %q", got, want)
	}

	got = crashMessage(42, "launchdeck dev")
	want = "launchdeck dev crashed: 42\n" + url
	if got != want {
		t.Errorf("non-string value:\n got %q\nwant %q", got, want)
	}

	// A multi-line panic value collapses to a single line — the message is
	// always exactly two lines.
	got = crashMessage("line1\nline2", "launchdeck dev")
	if lines := strings.Count(got, "\n"); lines != 1 {
		t.Errorf("multi-line value produced %d newlines, want exactly 1: %q", lines, got)
	}
	if !strings.Contains(got, "line1 line2") {
		t.Errorf("multi-line value not collapsed: %q", got)
	}
}

// noStart fails the test if the guards/TUI path is reached.
func noStart(t *testing.T) func() int {
	return func() int {
		t.Helper()
		t.Fatal("start (guards/TUI) reached — info flag should have returned first")
		return 0
	}
}

func TestRunVersionAndHelpReturnBeforeGuards(t *testing.T) {
	cases := [][]string{{"--version"}, {"-v"}, {"--help"}, {"-h"}, {"--version", "--help"}}
	for _, args := range cases {
		var out, errb bytes.Buffer
		code := run(args, &out, &errb, noStart(t))
		if code != 0 {
			t.Errorf("%v: code = %d, want 0", args, code)
		}
		if out.Len() == 0 {
			t.Errorf("%v: expected stdout output", args)
		}
	}
}

func TestRunHelpWinsOverVersion(t *testing.T) {
	var out bytes.Buffer
	run([]string{"--version", "--help"}, &out, io.Discard, noStart(t))
	if !strings.Contains(out.String(), "Usage: launchdeck [flags]") {
		t.Errorf("--version --help should print help, got %q", out.String())
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var errb bytes.Buffer
	code := run([]string{"--nope"}, io.Discard, &errb, func() int { return 0 })
	if code != 2 {
		t.Errorf("unknown flag: code = %d, want 2", code)
	}
	// The one-line hint is shown, not flag.PrintDefaults()'s -h/-v dump.
	if strings.Contains(errb.String(), "-version") || strings.Contains(errb.String(), "default") {
		t.Errorf("stderr leaked flag default dump: %q", errb.String())
	}
}

func TestRunRecoversMainGoroutinePanic(t *testing.T) {
	var errb bytes.Buffer
	code := run(nil, io.Discard, &errb, func() int { panic("boom") })
	if code != 1 {
		t.Errorf("panic: code = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "crashed: boom") ||
		!strings.Contains(errb.String(), "please report:") {
		t.Errorf("crash message not written to stderr: %q", errb.String())
	}
}

func TestRunNormalPathCallsStart(t *testing.T) {
	called := false
	code := run(nil, io.Discard, io.Discard, func() int { called = true; return 7 })
	if !called {
		t.Error("normal path did not call start")
	}
	if code != 7 {
		t.Errorf("code = %d, want 7 (start's return propagated)", code)
	}
}
