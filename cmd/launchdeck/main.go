// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 volkoffskij

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
	"github.com/volkoffskij/launchdeck/internal/session"
	ui "github.com/volkoffskij/launchdeck/internal/ui/bubbletea"
)

// version is overridden at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// buildVersion assembles the version line from the ldflags-injected version plus
// build-info values. It is pure so it can be table-tested. The returned string is
// already prefixed with "launchdeck ".
func buildVersion(version, mainVer, rev string, modified, hasInfo bool) string {
	if version != "dev" {
		return "launchdeck " + version
	}
	if hasInfo {
		if mainVer != "" && mainVer != "(devel)" {
			return "launchdeck " + mainVer // module-proxy install: go install <module>@<ver>
		}
		if rev != "" {
			r := rev
			if len(r) > 12 {
				r = r[:12]
			}
			if modified {
				r += "-dirty"
			}
			return "launchdeck dev (" + r + ")"
		}
	}
	return "launchdeck dev"
}

// versionString reads the real build info and delegates to buildVersion.
func versionString() string {
	if version != "dev" {
		return buildVersion(version, "", "", false, false)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return buildVersion(version, "", "", false, false)
	}
	var rev string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	return buildVersion(version, info.Main.Version, rev, modified, true)
}

// helpText is the hand-written --help output (not flag.PrintDefaults, which
// writes to stderr and dumps -h/-v default-value noise).
func helpText() string {
	return `launchdeck — a macOS launchctl services TUI

Usage: launchdeck [flags]

Flags:
  -h, --help     show this help and exit
  -v, --version  print the version and exit

Config files:
  ~/.config/launchdeck/session.json   restored UI session
  ~/.config/launchdeck/theme.json     colours and header toggle

Press ? inside the app for the full keymap.`
}

// crashMessage formats the two-line crash report. It is pure and deterministic:
// the panic value is rendered with %v and any newlines collapsed to spaces, so
// the message is always exactly two lines regardless of the value's type.
func crashMessage(v any, version string) string {
	val := strings.ReplaceAll(fmt.Sprintf("%v", v), "\n", " ")
	return version + " crashed: " + val +
		"\nplease report: https://github.com/volkoffskij/launchdeck/issues"
}

func main() {
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(os.Stderr, "launchdeck: macOS only")
		os.Exit(1)
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		fmt.Fprintln(os.Stderr, "launchdeck: launchctl not found in PATH")
		os.Exit(1)
	}

	uid := os.Getuid()
	var st app.AppState
	if p, err := session.Path(); err == nil {
		st = app.FromSession(session.Load(p), uid)
	} else {
		st = app.NewState(uid)
	}

	m := ui.New(st, launchctl.New())
	if p, err := ui.ThemePath(); err == nil {
		m = m.WithTheme(ui.LoadTheme(p))
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "launchdeck:", err)
		os.Exit(1)
	}
}
