package bubbletea

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/volkoffskij/launchdeck/internal/app"
	"github.com/volkoffskij/launchdeck/internal/i18n"
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

// TestWrapBodyByWord asserts long log/raw lines wrap at word boundaries, only
// hard-breaking a single token wider than the panel.
func TestWrapBodyByWord(t *testing.T) {
	spaced := "alpha beta gamma delta epsilon zeta eta theta"
	rows := strings.Split(wrapBody(spaced, 20), "\n")
	if len(rows) < 2 {
		t.Fatalf("expected the spaced line to wrap onto multiple rows, got %d", len(rows))
	}
	for i, r := range rows {
		if w := lipgloss.Width(r); w > 20 {
			t.Errorf("row %d width %d > 20", i, w)
		}
		// no row may start or end by splitting a word: a wrapped-at-space row,
		// trimmed, must consist of whole words from the input.
		for _, word := range strings.Fields(strings.TrimSpace(r)) {
			if !strings.Contains(spaced, word) {
				t.Errorf("row %d contains a split word %q", i, word)
			}
		}
	}

	// A single token longer than the panel is hard-broken (unavoidable), but
	// every piece still fits the width.
	long := "supercalifragilisticexpialidocious_plus_more"
	for i, r := range strings.Split(wrapBody(long, 20), "\n") {
		if w := lipgloss.Width(r); w > 20 {
			t.Errorf("long-token row %d width %d > 20", i, w)
		}
	}
}

// TestLogScrollNoRunaway reproduces the "scroll up lags" bug: scrolling down far
// past the end must not inflate Scroll.Log beyond the last page, so a single
// scroll-up immediately moves the window back.
func TestLogScrollNoRunaway(t *testing.T) {
	st := stateWithLogs(1, 60, 30, app.TabLogs)
	md := driveSized(st, 120, 40)

	// Scroll down way past the end (far more than the content).
	for i := 0; i < 100; i++ {
		next, _ := md.Update(app.ScrollMsg{Panel: app.FocusDetail, Delta: 5})
		md = next.(Model)
	}
	atBottom := md.st.Scroll.Log

	firstLogLine := func(m Model) string {
		for _, l := range strings.Split(m.View(), "\n") {
			if i := strings.Index(l, "LN"); i >= 0 {
				return l[i : i+7]
			}
		}
		return "NONE"
	}
	before := firstLogLine(md)

	// One scroll-up must move the visible window immediately.
	next, _ := md.Update(app.ScrollMsg{Panel: app.FocusDetail, Delta: -1})
	md = next.(Model)
	after := firstLogLine(md)

	if md.st.Scroll.Log != atBottom-1 {
		t.Errorf("scroll offset runaway: at bottom %d, after one up %d (want %d)",
			atBottom, md.st.Scroll.Log, atBottom-1)
	}
	if before == after {
		t.Errorf("scroll-up did not move the window immediately (before==after==%s)", before)
	}
}

// TestSidebarRowsDoNotWrap guards that long service labels stay on one row each
// (the box is exactly w x h) — the sidebar padding must be accounted for in the
// row width, or near-full-width labels wrap and shift the rows below.
func TestSidebarRowsDoNotWrap(t *testing.T) {
	rows := make([]app.RowVM, 10)
	for i := range rows {
		rows[i] = app.RowVM{
			Label:    fmt.Sprintf("com.apple.some.long.service.name.number%02d.agent", i),
			Running:  i%2 == 0,
			Selected: i == 3,
		}
	}
	vm := app.ListVM{Rows: rows}
	m := New(app.NewState(501), nil)
	m.Init()
	for _, w := range []int{20, 24, 30, 40, 48} {
		h := 14
		out := m.renderList(vm, w, h)
		if bw := lipgloss.Width(out); bw != w {
			t.Errorf("width %d: box width %d (want exactly %d)", w, bw, w)
		}
		if lines := len(strings.Split(out, "\n")); lines != h {
			t.Errorf("width %d: box height %d (want %d) — rows wrapped", w, lines, h)
		}
	}
}

// TestRawTabHasLineNumbers asserts the Raw tab renders an editor-style
// line-number gutter (1, 2, 3, …) down the left of the dump.
func TestRawTabHasLineNumbers(t *testing.T) {
	vm := app.DetailVM{
		Mode:      "ready",
		ActiveTab: app.TabRaw,
		Raw:       "first line\nsecond line\nthird line",
	}
	lines := detailLines(vm, 40, DefaultTheme())
	if len(lines) < 3 {
		t.Fatalf("expected 3 numbered rows, got %d", len(lines))
	}
	// The gutter is ANSI-shaded; compare on the plain (style-stripped) text.
	plain := func(s string) string { return strings.Join(strings.Fields(ansi.Strip(s)), " ") }
	if got := plain(lines[0]); !strings.HasPrefix(got, "1 first line") {
		t.Errorf("row 0 missing line number: %q", got)
	}
	if got := plain(lines[2]); !strings.HasPrefix(got, "3 third line") {
		t.Errorf("row 2 missing line number: %q", got)
	}
	// Every row still fits the panel width including the shaded gutter.
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 40 {
			t.Errorf("raw row %d width %d > 40", i, w)
		}
	}
}

// TestMetadataWrapsLongPath asserts the Metadata tab wraps a long plist path
// onto multiple rows (hard-broken, since a path has no spaces) instead of
// truncating it — and every row stays within the panel width.
func TestMetadataWrapsLongPath(t *testing.T) {
	longPath := "/Users/me/Library/LaunchAgents/" + strings.Repeat("a", 200) + ".plist"
	vm := app.DetailVM{
		Mode:      "ready",
		ActiveTab: app.TabMetadata,
		Label:     "com.x",
		Plist:     longPath,
	}
	lines := detailLines(vm, 40, DefaultTheme())
	joined := strings.Join(lines, "")
	// the full path's tail must be present (not truncated away)
	if !strings.Contains(strings.ReplaceAll(joined, " ", ""), strings.Repeat("a", 200)) {
		t.Errorf("long plist path was truncated, not wrapped")
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 40 {
			t.Errorf("metadata row %d width %d > 40", i, w)
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

// TestDividerDragResizesSidebar drives real mouse messages: press near the
// divider, drag left, and assert the sidebar width shrinks and is clamped to a
// safe minimum (never collapses the panels).
func TestDividerDragResizesSidebar(t *testing.T) {
	st := stateWithLogs(20, 0, 0, app.TabMetadata)
	md := driveSized(st, 100, 30)
	md.mouseOff = false   // mouse is off by default now; this test drives the captured mouse
	auto := md.sidebarW() // ~33 for width 100

	// Press on the divider column, then drag to a narrower X.
	next, _ := md.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: auto, Y: 5})
	md = next.(Model)
	if !md.dragging {
		t.Fatalf("press on divider (col %d) should start a drag", auto)
	}
	next, _ = md.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 25, Y: 5})
	md = next.(Model)
	if md.st.SidebarWidth != 25 {
		t.Errorf("drag to X=25 should set sidebar width 25, got %d", md.st.SidebarWidth)
	}

	// Drag way past the minimum: it must pin, not collapse.
	next, _ = md.Update(tea.MouseMsg{Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft, X: 1, Y: 5})
	md = next.(Model)
	if md.st.SidebarWidth != app.MinSidebarWidth {
		t.Errorf("drag past min should pin at %d, got %d", app.MinSidebarWidth, md.st.SidebarWidth)
	}

	// Release ends the drag; a later motion doesn't resize.
	next, _ = md.Update(tea.MouseMsg{Action: tea.MouseActionRelease, X: 1, Y: 5})
	md = next.(Model)
	if md.dragging {
		t.Error("release should end the drag")
	}
	pinned := md.st.SidebarWidth
	next, _ = md.Update(tea.MouseMsg{Action: tea.MouseActionMotion, X: 50, Y: 5})
	md = next.(Model)
	if md.st.SidebarWidth != pinned {
		t.Errorf("motion after release should not resize: %d -> %d", pinned, md.st.SidebarWidth)
	}
	_ = auto
}

func TestMouseToggleCapture(t *testing.T) {
	m := New(app.NewState(501), nil)
	m.Init()
	// Mouse is OFF by default so a plain drag selects text like a text editor.
	if !m.mouseOff {
		t.Fatal("mouse should be off by default")
	}
	var md tea.Model = m
	// while off, a mouse event is ignored (terminal owns selection)
	md0, _ := md.(Model).handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: 5, Y: 5})
	if md0.(Model).st.Selected != md.(Model).st.Selected {
		t.Error("mouse events must be ignored while capture is off")
	}
	// press 'm' -> mouse captured
	md, cmd := md.(Model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if md.(Model).mouseOff {
		t.Fatal("m should capture the mouse")
	}
	if cmd == nil {
		t.Error("m should emit an EnableMouse command")
	}
	// press 'm' again -> released
	md, cmd = md.(Model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !md.(Model).mouseOff {
		t.Fatal("second m should release the mouse")
	}
	if cmd == nil {
		t.Error("second m should emit a DisableMouse command")
	}
}

func TestAffectsDetailScroll(t *testing.T) {
	// sidebar scroll must NOT trigger the expensive detail re-wrap
	if affectsDetailScroll(app.ScrollMsg{Panel: app.FocusSidebar}) {
		t.Error("sidebar scroll should not affect detail scroll")
	}
	if !affectsDetailScroll(app.ScrollMsg{Panel: app.FocusDetail}) {
		t.Error("detail scroll should affect detail scroll")
	}
	if !affectsDetailScroll(app.SetTab{Tab: app.TabRaw}) {
		t.Error("tab change should affect detail scroll")
	}
	if affectsDetailScroll(app.SetFilter{}) {
		t.Error("filter change should not affect detail scroll")
	}
}

func TestSidebarWheelScrollsList(t *testing.T) {
	st := stateWithLogs(60, 0, 0, app.TabMetadata) // 60 services, small viewport
	md := driveSized(st, 100, 20)
	before := md.st.Scroll.List
	// wheel down over a sidebar row
	next, _ := md.Update(app.ScrollMsg{Panel: app.FocusSidebar, Delta: 3})
	md = next.(Model)
	if md.st.Scroll.List <= before {
		t.Errorf("sidebar wheel should scroll the list: %d -> %d", before, md.st.Scroll.List)
	}
}

func TestLogsHaveLineNumbers(t *testing.T) {
	vm := app.DetailVM{
		Mode: "ready", ActiveTab: app.TabLogs,
		LogLines: []string{"[out] hello", "[err] boom", "[out] third"},
	}
	lines := detailLines(vm, 50, DefaultTheme())
	if len(lines) < 3 {
		t.Fatalf("want 3 numbered log rows, got %d", len(lines))
	}
	// Logs are newest-first, so numbers descend: top = 3, bottom = 1.
	if !strings.HasPrefix(strings.TrimLeft(ansi.Strip(lines[0]), " "), "3 ") {
		t.Errorf("top log line should be numbered 3 (newest): %q", ansi.Strip(lines[0]))
	}
	if !strings.HasPrefix(strings.TrimLeft(ansi.Strip(lines[2]), " "), "1 ") {
		t.Errorf("bottom log line should be numbered 1 (oldest): %q", ansi.Strip(lines[2]))
	}
}

func TestLogTagsColorCoded(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	out := colorLogTags("[out] normal\n[err] error", DefaultTheme())
	lines := strings.Split(out, "\n")
	outSGR := firstSGR(lines[0])
	errSGR := firstSGR(lines[1])
	if outSGR == "" || errSGR == "" {
		t.Fatalf("both tags should be colored: out=%q err=%q", lines[0], lines[1])
	}
	if outSGR == errSGR {
		t.Errorf("out and err tags should differ in color: both %q", outSGR)
	}
}

// firstSGR returns the first SGR foreground escape (…m) in s.
func firstSGR(s string) string {
	i := strings.Index(s, "\x1b[38")
	if i < 0 {
		return ""
	}
	if end := strings.IndexByte(s[i:], 'm'); end >= 0 {
		return s[i : i+end+1]
	}
	return ""
}

func TestHeaderToggle(t *testing.T) {
	for _, hdr := range []bool{true, false} {
		th := DefaultTheme()
		th.Header = hdr
		m := New(app.NewState(501), nil).WithTheme(th)
		m.Init()
		var md tea.Model = m
		md, _ = md.(Model).Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		out := md.(Model).View()
		if w, h := lipgloss.Width(out), len(strings.Split(out, "\n")); w > 100 || h > 30 {
			t.Errorf("header=%v overflow: %dx%d", hdr, w, h)
		}
		firstRow := strings.Split(out, "\n")[0]
		if hdr && !strings.Contains(firstRow, "LaunchDeck") {
			t.Errorf("header on: title expected in first row, got %q", ansi.Strip(firstRow))
		}
		if !hdr && strings.Contains(firstRow, "LaunchDeck — launchctl") {
			t.Error("header off: title should not be shown")
		}
	}
}

func TestHelpOverlayEnglishGolden(t *testing.T) {
	i18n.SetLang(i18n.En)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })
	m := driveSized(app.NewState(501), 100, 40)
	m.helpOpen = true
	out := m.render()
	for _, want := range []string{
		"LaunchDeck — help",
		"Navigation",
		"switch focus: sidebar ↔ detail",
		"press ? or Esc to close",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing %q", want)
		}
	}
}

func TestRussianRenderNoOverflow(t *testing.T) {
	i18n.SetLang(i18n.Ru)
	t.Cleanup(func() { i18n.SetLang(i18n.En) })
	const w, h = 60, 20 // the documented minimum

	assertBounds := func(t *testing.T, out string) {
		lines := strings.Split(out, "\n")
		if len(lines) > h {
			t.Fatalf("frame has %d lines, want ≤ %d", len(lines), h)
		}
		for i, l := range lines {
			if lipgloss.Width(l) > w {
				t.Errorf("line %d width %d > %d: %q", i, lipgloss.Width(l), w, l)
			}
		}
	}

	// Each case renders the risky Cyrillic-width regions this task changed: the
	// dynamically aligned metadata block (metaBlock), the localized tab row, the
	// long-path metadata, the Logs tab, and the help overlay. A service must be
	// selected or renderDetail short-circuits on "empty" and never draws them.
	t.Run("metadata", func(t *testing.T) {
		assertBounds(t, driveSized(stateWithLogs(1, 0, 0, app.TabMetadata), w, h).render())
	})
	t.Run("logs", func(t *testing.T) {
		assertBounds(t, driveSized(stateWithLogs(1, 20, 40, app.TabLogs), w, h).render())
	})
	t.Run("help", func(t *testing.T) {
		m := driveSized(stateWithLogs(1, 0, 0, app.TabMetadata), w, h)
		m.helpOpen = true
		assertBounds(t, m.render())
	})
}

func TestHelpOverlayToggle(t *testing.T) {
	m := New(app.NewState(501), nil)
	m.Init()
	var md tea.Model = m
	md, _ = md.(Model).Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	md, _ = md.(Model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !md.(Model).helpOpen {
		t.Fatal("? should open the help overlay")
	}
	out := md.(Model).View()
	if !strings.Contains(ansi.Strip(out), "LaunchDeck — help") {
		t.Error("help overlay should show the help title")
	}
	if w, h := lipgloss.Width(out), len(strings.Split(out, "\n")); w > 100 || h > 32 {
		t.Errorf("help overlay overflow: %dx%d", w, h)
	}
	// a random key does not close it; esc does
	md, _ = md.(Model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if !md.(Model).helpOpen {
		t.Error("a non-close key should leave help open")
	}
	md, _ = md.(Model).handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if md.(Model).helpOpen {
		t.Error("esc should close help")
	}
}
