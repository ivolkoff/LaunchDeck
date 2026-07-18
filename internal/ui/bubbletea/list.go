package bubbletea

import (
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/volkoffskij/launchdeck/internal/app"
)

// renderList renders a box whose TOTAL OUTER size (including the border and its
// 1-col inner padding) is exactly w x h. vm.Rows is already the windowed set of
// rows (see app.Derive) — this just styles and prints them. The 1-col padding
// keeps the selected-row highlight off the border instead of flush against it.
func (m Model) renderList(vm app.ListVM, w, h int) string {
	th := m.theme
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(th.border()).
		Padding(0, 1)
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
		return style.Render(th.muted().Render(vm.Placeholder))
	}
	var b []string
	for _, r := range vm.Rows {
		dotStyle := th.stopped()
		dot := "○"
		if r.Running {
			dot, dotStyle = "●", th.running()
		}
		label := ellipsize(r.Label, contentW-2)
		line := dotStyle.Render(dot) + " " + label
		if r.Gone {
			line += th.gone().Render(" (gone)")
		}
		if r.Selected {
			// Fill the row to the content width so the highlight spans it evenly,
			// then the box's padding gives the gap from the border.
			line = th.sel().Width(contentW).Render(dot + " " + label)
		}
		b = append(b, zone.Mark("row:"+r.Label, line))
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
