package bubbletea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

const (
	minWidth  = 60
	minHeight = 20
)

func (m Model) render() string {
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			"terminal too small (need ≥60×20)")
	}
	vm := app.Derive(m.st)
	sidebarW := m.sidebarW()
	detailW := m.width - sidebarW - 1
	bodyH := m.height - 1 // status row

	sidebar := m.renderList(vm.List, sidebarW, bodyH)
	detail := m.renderDetail(vm.Detail, detailW, bodyH, m.st.Scroll.Log)
	divider := zone.Mark("divider", " ")
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, divider, detail)
	status := m.renderStatus(vm.Status, m.width)
	frame := zone.Scan(lipgloss.JoinVertical(lipgloss.Left, body, status))
	// Hard final gate: no sub-renderer bug can push the frame past the terminal.
	// zone.Scan already stripped the zone markers, so the frame is plain ANSI+text
	// and safe to clamp line-by-line. This is a belt-and-suspenders guarantee on
	// top of each renderer's own width/height budgeting.
	return clampFrame(frame, m.width, m.height)
}

// clampFrame forces s to occupy at most w columns and h rows: every line is
// truncated to w display cells (ANSI-aware), and at most the first h lines are
// kept. It never pads — a frame that is already within bounds is returned
// unchanged in extent.
func clampFrame(s string, w, h int) string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	trunc := lipgloss.NewStyle().MaxWidth(w)
	for i, l := range lines {
		if lipgloss.Width(l) > w {
			lines[i] = trunc.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// sidebarWidthAuto is the default sidebar outer width (a third, clamped to
// [24, 48]) used when the user hasn't dragged the divider.
func sidebarWidthAuto(width int) int {
	return clampInt(int(float64(width)*0.33), 24, 48)
}

// sidebarW is the sidebar's actual outer width: the user-dragged width if set
// (reduce keeps it clamped to a safe range), else the auto third.
func (m Model) sidebarW() int {
	if m.st.SidebarWidth > 0 {
		return m.st.SidebarWidth
	}
	return sidebarWidthAuto(m.width)
}

// detailContentW is the inner (inside-border) column budget of the detail panel
// — the width the log/raw body is wrapped to. Kept in sync with render()'s own
// sidebar/detail split so the scroll clamp wraps to exactly what is drawn.
func (m Model) detailContentW() int {
	frame := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).GetHorizontalFrameSize()
	w := m.width - m.sidebarW() - 1 - frame
	if w < 1 {
		w = 1
	}
	return w
}

// clampLogScroll bounds Scroll.Log to the active tab's wrapped content: at most
// max(0, displayLines - logViewportRows). Returns the clamped offset; never
// inflates. Uses the same detailLines the renderer windows, so they agree.
func (m Model) clampLogScroll() int {
	if m.st.Scroll.Log == 0 {
		return 0 // already valid; nothing to clamp down to
	}
	_, logH := viewportHeights(m.height)
	lines := len(m.detailCache) // the cache is fresh whenever this runs
	maxStart := lines - logH
	if maxStart < 0 {
		maxStart = 0
	}
	s := m.st.Scroll.Log
	if s < 0 {
		s = 0
	}
	if s > maxStart {
		s = maxStart
	}
	return s
}

// viewportHeights returns the content-row budgets renderList and
// renderDetail's Logs/Raw tabs end up with for a terminal of the given
// height. model.go's WindowSizeMsg handler feeds these into reduce so
// Scroll.List/Scroll.Log windowing agrees with what render() actually draws.
func viewportHeights(height int) (listH, logH int) {
	bodyH := height - 1 // status row
	frame := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).GetVerticalFrameSize()
	listH = bodyH - frame
	if listH < 1 {
		listH = 1
	}
	logH = listH - 1 // tabs line
	if logH < 1 {
		logH = 1
	}
	return
}
