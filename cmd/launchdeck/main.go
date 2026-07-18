package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
	"github.com/volkoffskij/launchdeck/internal/session"
	ui "github.com/volkoffskij/launchdeck/internal/ui/bubbletea"
)

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
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "launchdeck:", err)
		os.Exit(1)
	}
}
