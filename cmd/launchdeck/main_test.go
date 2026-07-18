package main

import (
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
}
