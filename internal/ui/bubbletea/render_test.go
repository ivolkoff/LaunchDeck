package bubbletea

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/launchctl"
)

// driveSized builds a Model, runs Init (arms bubblezone), and applies a window
// size — the same path Bubble Tea drives at runtime. The client is nil; tests
// never execute returned Cmds, only inspect View().
func driveSized(st app.AppState, w, h int) Model {
	m := New(st, nil)
	m.Init()
	var md tea.Model = m
	md, _ = md.(Model).Update(tea.WindowSizeMsg{Width: w, Height: h})
	return md.(Model)
}

func stateWithLogs(nServices, nLogLines, lineLen int, tab app.Tab) app.AppState {
	st := app.NewState(501)
	var svcs []launchctl.Service
	for i := 0; i < nServices; i++ {
		svcs = append(svcs, launchctl.Service{
			Label:  fmt.Sprintf("com.test.svc%03d", i),
			Domain: launchctl.GUIDomain(501),
		})
	}
	st = app.Reduce(app.ServicesLoaded{Services: svcs}, st)
	if nServices == 0 {
		return st
	}
	sel := svcs[0].Label
	st = app.Reduce(app.SelectService{Label: sel}, st)
	det := launchctl.ServiceDetail{
		Service:    launchctl.Service{Label: sel, Domain: launchctl.GUIDomain(501)},
		StdoutPath: "/tmp/a.out",
		Program:    "/very/long/path/" + strings.Repeat("p", 300),
		PlistPath:  "/very/long/path/" + strings.Repeat("q", 300),
		Raw:        strings.Repeat("R"+strings.Repeat("z", 400)+"\n", 40),
	}
	st = app.Reduce(app.ServiceDetailLoaded{Target: det.Domain.Target(sel), Detail: det}, st)
	var lines []app.LogLine
	for i := 0; i < nLogLines; i++ {
		lines = append(lines, app.LogLine{
			Stream: "out",
			Text:   fmt.Sprintf("LN%04d ", i) + strings.Repeat("x", lineLen),
		})
	}
	st = app.Reduce(app.LogLinesAppended{TailTarget: det.Domain.Target(sel), Lines: lines}, st)
	st.ActiveTab = tab
	return st
}

// TestViewNeverOverflows is a regression guard for a bug that recurred twice:
// the rendered frame must never exceed the terminal's width or height. Long log
// lines (which lipgloss would otherwise WRAP onto extra rows), long metadata
// paths, a long status message, and a long prompt are the known triggers.
func TestViewNeverOverflows(t *testing.T) {
	cases := []struct {
		name          string
		services      int
		logs, lineLen int
		tab           app.Tab
		statusMsg     string
		longPrompt    bool
	}{
		{"empty", 0, 0, 0, app.TabMetadata, "", false},
		{"many services", 200, 0, 0, app.TabMetadata, "", false},
		{"logs many short lines", 1, 200, 20, app.TabLogs, "", false},
		{"logs few long lines (wrap trap)", 1, 20, 400, app.TabLogs, "", false},
		{"logs many long lines", 1, 200, 400, app.TabLogs, "", false},
		{"raw long lines", 1, 5, 20, app.TabRaw, "", false},
		{"metadata long paths", 1, 5, 20, app.TabMetadata, "", false},
		{"long status message", 1, 5, 20, app.TabMetadata, "failed: " + strings.Repeat("E", 500), false},
		{"long sudo prompt", 1, 5, 20, app.TabLogs, "", true},
	}
	sizes := [][2]int{{60, 20}, {80, 24}, {120, 40}, {200, 50}}
	for _, c := range cases {
		for _, sz := range sizes {
			st := stateWithLogs(c.services, c.logs, c.lineLen, c.tab)
			if c.statusMsg != "" {
				st.StatusMsg = c.statusMsg
			}
			if c.longPrompt {
				st.PendingSudo = app.PendingSudo{Active: true, Target: strings.Repeat("T", 400)}
			}
			out := driveSized(st, sz[0], sz[1]).View()
			w := lipgloss.Width(out)
			h := len(strings.Split(out, "\n"))
			if w > sz[0] || h > sz[1] {
				t.Errorf("%s at %dx%d: rendered %dx%d exceeds budget",
					c.name, sz[0], sz[1], w, h)
			}
		}
	}
}

// TestClampFrameHardBounds proves the final gate is absolute: whatever a
// sub-renderer produces (over-wide lines, too many rows), the frame is forced
// into w x h. This is the guarantee the layout can't overflow regardless of any
// other bug.
func TestClampFrameHardBounds(t *testing.T) {
	inputs := []string{
		strings.Repeat("x", 500),                                   // one over-wide line
		strings.Repeat(strings.Repeat("y", 300)+"\n", 200),         // many over-wide lines
		strings.Join(make([]string, 100), "row of text padding\n"), // too many rows
		"\x1b[31m" + strings.Repeat("z", 400) + "\x1b[0m",          // ANSI-colored over-wide
	}
	for _, w := range []int{1, 40, 80, 120} {
		for _, h := range []int{1, 20, 40} {
			for i, in := range inputs {
				out := clampFrame(in, w, h)
				lines := strings.Split(out, "\n")
				if len(lines) > h {
					t.Errorf("input %d at %dx%d: %d lines > %d", i, w, h, len(lines), h)
				}
				for _, l := range lines {
					if lipgloss.Width(l) > w {
						t.Errorf("input %d at %dx%d: line width %d > %d", i, w, h, lipgloss.Width(l), w)
					}
				}
			}
		}
	}
}

// TestLogScrollMovesWindow guards that scrolling the detail panel actually
// changes which log lines are shown (the offset is applied in the render, not
// just stored in state).
func TestLogScrollMovesWindow(t *testing.T) {
	st := stateWithLogs(1, 200, 400, app.TabLogs)
	md := driveSized(st, 120, 40)

	firstLogLine := func(m Model) string {
		for _, l := range strings.Split(m.View(), "\n") {
			if i := strings.Index(l, "LN"); i >= 0 {
				return l[i : i+7]
			}
		}
		return "NONE"
	}

	before := firstLogLine(md)
	if before == "NONE" {
		t.Fatal("no log line rendered before scroll")
	}
	next, _ := md.Update(app.ScrollMsg{Panel: app.FocusDetail, Delta: 20})
	after := firstLogLine(next.(Model))
	if before == after {
		t.Errorf("log scroll did nothing: still at %s", before)
	}
}
