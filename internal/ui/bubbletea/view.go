package bubbletea

import (
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
	sidebarW := clampInt(int(float64(m.width)*0.33), 24, 48)
	detailW := m.width - sidebarW - 1
	bodyH := m.height - 1 // status row

	sidebar := renderList(vm.List, sidebarW, bodyH, m.st.Focus == app.FocusSidebar)
	detail := renderDetail(vm.Detail, detailW, bodyH, m.st.Scroll.Log)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", detail)
	status := renderStatus(vm.Status, m.width)
	return zone.Scan(lipgloss.JoinVertical(lipgloss.Left, body, status))
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
