package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

// renderList renders a box whose TOTAL OUTER size (including the border) is
// exactly w x h. vm.Rows is already the windowed set of rows to display
// (see app.Derive) — this just prints them.
func renderList(vm app.ListVM, w, h int, focused bool) string {
	border := lipgloss.NormalBorder()
	style := lipgloss.NewStyle().Border(border)
	contentW := w - style.GetHorizontalFrameSize()
	contentH := h - style.GetVerticalFrameSize()
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}
	style = style.Width(contentW).Height(contentH)
	if vm.Placeholder != "" {
		return style.Render(vm.Placeholder)
	}
	var b []string
	for _, r := range vm.Rows {
		dot := "○"
		if r.Running {
			dot = "●"
		}
		line := dot + " " + ellipsize(r.Label, contentW-4)
		if r.Gone {
			line += " (gone)"
		}
		row := lipgloss.NewStyle().Render(line)
		if r.Selected {
			row = lipgloss.NewStyle().Reverse(true).Render(line)
		}
		b = append(b, zone.Mark("row:"+r.Label, row))
	}
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, b...))
}

func ellipsize(s string, max int) string {
	if max < 1 {
		max = 1
	}
	if len(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return s[:max-1] + "…"
}
